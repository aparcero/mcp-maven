package mavencentral

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aparcero/mcp-maven/internal/cache"
	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/observability"
)

// ErrNotFound indicates Maven Central has no metadata for the artifact.
var ErrNotFound = errors.New("artifact not found")

// Client represents a Maven Central repository client.
type Client struct {
	httpClient *http.Client
	config     config.MavenCentralConfig
	cacheCfg   config.CacheConfig
	cache      *cache.Cache
	keyGen     cache.Key
	comparator *domain.VersionComparator
}

// NewClient creates a new Maven Central client.
func NewClient(cfg config.MavenCentralConfig, c *cache.Cache, cacheCfg ...config.CacheConfig) *Client {
	cacheConfig := defaultCacheConfig()
	if len(cacheCfg) > 0 {
		cacheConfig = withCacheDefaults(cacheCfg[0])
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		config:     cfg,
		cacheCfg:   cacheConfig,
		cache:      c,
		keyGen:     cache.Key{},
		comparator: domain.NewVersionComparator(),
	}
}

// IsNotFound returns true when an error represents a missing Maven artifact.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func defaultCacheConfig() config.CacheConfig {
	return config.CacheConfig{
		AllVersionsTTL:    time.Hour,
		VersionChecksTTL:  6 * time.Hour,
		HistoricalDataTTL: 24 * time.Hour,
		TimestampCacheTTL: 24 * time.Hour,
	}
}

func withCacheDefaults(cfg config.CacheConfig) config.CacheConfig {
	defaults := defaultCacheConfig()
	if cfg.AllVersionsTTL == 0 {
		cfg.AllVersionsTTL = defaults.AllVersionsTTL
	}
	if cfg.VersionChecksTTL == 0 {
		cfg.VersionChecksTTL = defaults.VersionChecksTTL
	}
	if cfg.HistoricalDataTTL == 0 {
		cfg.HistoricalDataTTL = defaults.HistoricalDataTTL
	}
	if cfg.TimestampCacheTTL == 0 {
		cfg.TimestampCacheTTL = defaults.TimestampCacheTTL
	}
	return cfg
}

// MavenMetadata represents the maven-metadata.xml structure.
type MavenMetadata struct {
	XMLName    xml.Name   `xml:"metadata"`
	GroupID    string     `xml:"groupId"`
	Artifact   string     `xml:"artifactId"`
	Versioning Versioning `xml:"versioning"`
}

// Versioning contains version information.
type Versioning struct {
	Latest      string   `xml:"latest"`
	Release     string   `xml:"release"`
	Versions    Versions `xml:"versions"`
	LastUpdated string   `xml:"lastUpdated"`
}

// Versions contains the list of available versions.
type Versions struct {
	Version []string `xml:"version"`
}

// HasValidVersioning checks if the metadata has valid versioning information.
func (m *MavenMetadata) HasValidVersioning() bool {
	return len(m.Versioning.Versions.Version) > 0
}

// GetAllVersions fetches all versions for a Maven coordinate.
func (c *Client) GetAllVersions(ctx context.Context, coord *domain.Coordinates) ([]string, error) {
	// Check cache first
	cacheKey := c.keyGen.AllVersions(coord.GroupID, coord.ArtifactID, coord.Packaging)
	if cached, found := c.cache.Get(cacheKey); found {
		if versions, ok := cached.([]string); ok {
			return versions, nil
		}
	}

	// Fetch from Maven Central
	metadata, err := c.fetchMetadata(ctx, coord)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	if !metadata.HasValidVersioning() {
		return []string{}, nil
	}

	// Sort versions in descending order
	versions := c.sortVersions(metadata.Versioning.Versions.Version)

	// Limit results
	if len(versions) > c.config.MaxResults {
		versions = versions[:c.config.MaxResults]
	}

	// Cache the results
	c.cache.Set(cacheKey, versions, c.cacheCfg.AllVersionsTTL)

	return versions, nil
}

// GetVersionsWithTimestamps fetches versions with accurate POM timestamps.
func (c *Client) GetVersionsWithTimestamps(ctx context.Context, coord *domain.Coordinates, limit int) ([]domain.MavenArtifact, error) {
	cacheKey := c.keyGen.HistoricalData(coord.GroupID, coord.ArtifactID, limit, coord.Packaging)
	if cached, found := c.cache.Get(cacheKey); found {
		if artifacts, ok := cached.([]domain.MavenArtifact); ok {
			return artifacts, nil
		}
	}

	allVersions, err := c.GetAllVersions(ctx, coord)
	if err != nil {
		return nil, err
	}

	if len(allVersions) == 0 {
		return []domain.MavenArtifact{}, nil
	}

	// Limit to requested number
	if limit > 0 && limit < len(allVersions) {
		allVersions = allVersions[:limit]
	}

	// Fetch timestamps for each version
	artifacts := make([]domain.MavenArtifact, 0, len(allVersions))
	for _, version := range allVersions {
		timestamp, err := c.fetchTimestamp(ctx, coord, version)
		if err != nil || timestamp <= 0 {
			observability.Debug("failed to fetch timestamp", "coordinate", coord.String(), "version", version, "error", err)
			continue
		}

		coordWithVersion := *coord
		coordWithVersion.Version = version

		artifacts = append(artifacts, domain.MavenArtifact{
			Coordinate: coordWithVersion.String(),
			GroupID:    coord.GroupID,
			ArtifactID: coord.ArtifactID,
			Version:    version,
			Packaging:  coord.Packaging,
			Timestamp:  timestamp,
		})
	}

	// Sort by version descending
	c.sortArtifactsByVersion(artifacts)

	if len(artifacts) > 0 {
		c.cache.Set(cacheKey, artifacts, c.cacheCfg.HistoricalDataTTL)
	}

	return artifacts, nil
}

// GetArtifactWithTimestamp fetches timestamp metadata for one exact artifact version.
func (c *Client) GetArtifactWithTimestamp(ctx context.Context, coord *domain.Coordinates, version string) (domain.MavenArtifact, error) {
	if version == "" {
		return domain.MavenArtifact{}, fmt.Errorf("version required for timestamp lookup")
	}

	timestamp, err := c.fetchTimestamp(ctx, coord, version)
	if err != nil {
		return domain.MavenArtifact{}, err
	}
	if timestamp <= 0 {
		return domain.MavenArtifact{}, fmt.Errorf("%w: %s:%s:%s", ErrNotFound, coord.GroupID, coord.ArtifactID, version)
	}

	coordWithVersion := *coord
	coordWithVersion.Version = version

	return domain.MavenArtifact{
		Coordinate: coordWithVersion.String(),
		GroupID:    coord.GroupID,
		ArtifactID: coord.ArtifactID,
		Version:    version,
		Packaging:  coord.Packaging,
		Timestamp:  timestamp,
	}, nil
}

// VersionExists checks if a specific version exists.
func (c *Client) VersionExists(ctx context.Context, coord *domain.Coordinates, version string) (bool, error) {
	// Check cache first
	cacheKey := c.keyGen.VersionCheck(coord.GroupID, coord.ArtifactID, version, coord.Packaging)
	if cached, found := c.cache.Get(cacheKey); found {
		if exists, ok := cached.(bool); ok {
			return exists, nil
		}
	}

	// Fetch all versions and check
	versions, err := c.GetAllVersions(ctx, coord)
	if err != nil {
		return false, err
	}

	exists := false
	for _, v := range versions {
		if v == version {
			exists = true
			break
		}
	}

	// Cache the result
	c.cache.Set(cacheKey, exists, c.cacheCfg.VersionChecksTTL)

	return exists, nil
}

// GetLatestVersion fetches the latest version.
func (c *Client) GetLatestVersion(ctx context.Context, coord *domain.Coordinates, filter domain.StabilityFilter) (string, error) {
	versions, err := c.GetAllVersions(ctx, coord)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", nil
	}

	// Apply stability filter
	filtered := c.applyStabilityFilter(versions, filter)
	if len(filtered) == 0 {
		return "", nil
	}

	return filtered[0], nil
}

// applyStabilityFilter filters versions based on stability preference.
func (c *Client) applyStabilityFilter(versions []string, filter domain.StabilityFilter) []string {
	var stable, other []string

	for _, v := range versions {
		if c.comparator.IsStable(v) {
			stable = append(stable, v)
		} else {
			other = append(other, v)
		}
	}

	switch filter {
	case domain.StabilityStableOnly:
		return stable
	case domain.StabilityAll:
		return versions
	case domain.StabilityPreferStable:
		if len(stable) > 0 {
			return stable
		}
		return other
	default:
		return versions
	}
}

// fetchMetadata fetches and parses maven-metadata.xml.
func (c *Client) fetchMetadata(ctx context.Context, coord *domain.Coordinates) (*MavenMetadata, error) {
	url := c.buildMetadataURL(coord)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, coord.String())
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metadata MavenMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata XML: %w", err)
	}

	return &metadata, nil
}

// fetchTimestamp fetches the timestamp (Last-Modified header) for a POM file.
func (c *Client) fetchTimestamp(ctx context.Context, coord *domain.Coordinates, version string) (int64, error) {
	url := c.buildPomURL(coord, version)

	// Check cache first
	cacheKey := c.keyGen.Timestamp(coord.GroupID, coord.ArtifactID, version)
	if cached, found := c.cache.Get(cacheKey); found {
		if ts, ok := cached.(int64); ok {
			return ts, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse Last-Modified header
	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		return 0, nil
	}

	timestamp, err := http.ParseTime(lastModified)
	if err != nil {
		return 0, err
	}

	ts := timestamp.UnixMilli()

	// Cache the timestamp
	c.cache.Set(cacheKey, ts, c.cacheCfg.TimestampCacheTTL)

	return ts, nil
}

// buildMetadataURL builds the URL to maven-metadata.xml.
func (c *Client) buildMetadataURL(coord *domain.Coordinates) string {
	groupPath := strings.ReplaceAll(coord.GroupID, ".", "/")
	return fmt.Sprintf("%s/%s/%s/maven-metadata.xml",
		c.config.RepositoryBaseURL, groupPath, coord.ArtifactID)
}

// buildPomURL builds the URL to a POM file.
func (c *Client) buildPomURL(coord *domain.Coordinates, version string) string {
	groupPath := strings.ReplaceAll(coord.GroupID, ".", "/")
	filename := fmt.Sprintf("%s-%s.pom", coord.ArtifactID, version)
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		c.config.RepositoryBaseURL, groupPath, coord.ArtifactID, version, filename)
}

// sortVersions sorts versions in descending order (newest first).
func (c *Client) sortVersions(versions []string) []string {
	sorted := make([]string, len(versions))
	copy(sorted, versions)

	// Use the comparator to sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if c.comparator.Compare(sorted[j], sorted[i]) > 0 {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// sortArtifactsByVersion sorts artifacts by version descending.
func (c *Client) sortArtifactsByVersion(artifacts []domain.MavenArtifact) {
	for i := 0; i < len(artifacts); i++ {
		for j := i + 1; j < len(artifacts); j++ {
			if c.comparator.Compare(artifacts[j].Version, artifacts[i].Version) > 0 {
				artifacts[i], artifacts[j] = artifacts[j], artifacts[i]
			}
		}
	}
}

// GetLicenses extracts license information from a POM file.
func (c *Client) GetLicenses(ctx context.Context, coord *domain.Coordinates) ([]LicenseInfo, error) {
	if coord.Version == "" {
		return nil, fmt.Errorf("version required for license lookup")
	}

	url := c.buildPomURL(coord, coord.Version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return c.parseLicensesFromPOM(string(body)), nil
}

// LicenseInfo represents license information from a POM.
type LicenseInfo struct {
	Name string
	URL  string
}

// parseLicensesFromPOM extracts license information from POM XML content.
func (c *Client) parseLicensesFromPOM(pomContent string) []LicenseInfo {
	// Simple regex-based extraction
	// In production, would use proper XML parsing
	licenses := []LicenseInfo{}

	// Find all <license> sections
	licenseStart := 0
	for {
		startIdx := strings.Index(pomContent[licenseStart:], "<license>")
		if startIdx == -1 {
			break
		}
		startIdx += licenseStart

		endIdx := strings.Index(pomContent[startIdx:], "</license>")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx + len("</license>")

		licenseBlock := pomContent[startIdx:endIdx]

		// Extract name
		nameStart := strings.Index(licenseBlock, "<name>")
		name := ""
		if nameStart != -1 {
			nameStart += len("<name>")
			nameEnd := strings.Index(licenseBlock[nameStart:], "</name>")
			if nameEnd != -1 {
				name = strings.TrimSpace(licenseBlock[nameStart : nameStart+nameEnd])
			}
		}

		// Extract URL
		urlStart := strings.Index(licenseBlock, "<url>")
		url := ""
		if urlStart != -1 {
			urlStart += len("<url>")
			urlEnd := strings.Index(licenseBlock[urlStart:], "</url>")
			if urlEnd != -1 {
				url = strings.TrimSpace(licenseBlock[urlStart : urlStart+urlEnd])
			}
		}

		if name != "" {
			licenses = append(licenses, LicenseInfo{Name: name, URL: url})
		}

		licenseStart = endIdx
	}

	return licenses
}
