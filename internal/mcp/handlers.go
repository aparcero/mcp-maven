package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/mavencentral"
	"github.com/aparcero/mcp-maven/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetLatestVersionArgs represents arguments for get_latest_version.
type GetLatestVersionArgs struct {
	GroupID         string                 `json:"groupId" jsonschema:"required,The Maven groupId (e.g., 'org.springframework')"`
	ArtifactID      string                 `json:"artifactId" jsonschema:"required,The Maven artifactId (e.g., 'spring-core')"`
	StabilityFilter domain.StabilityFilter `json:"stabilityFilter" jsonschema:"The stability filter: ALL, STABLE_ONLY, or PREFER_STABLE (default: PREFER_STABLE)"`
}

// CheckVersionExistsArgs represents arguments for check_version_exists.
type CheckVersionExistsArgs struct {
	GroupID    string `json:"groupId" jsonschema:"required,The Maven groupId"`
	ArtifactID string `json:"artifactId" jsonschema:"required,The Maven artifactId"`
	Version    string `json:"version" jsonschema:"required,The version to check"`
}

// CheckMultipleDependenciesArgs represents arguments for check_multiple_dependencies.
type CheckMultipleDependenciesArgs struct {
	Dependencies []DependencySpec `json:"dependencies" jsonschema:"required,List of dependencies to check"`
}

// DependencySpec represents a single dependency specification.
type DependencySpec struct {
	GroupID    string `json:"groupId" jsonschema:"required,The Maven groupId"`
	ArtifactID string `json:"artifactId" jsonschema:"required,The Maven artifactId"`
	Version    string `json:"version,omitempty" jsonschema:"Optional version to check"`
}

// CompareDependencyVersionsArgs represents arguments for compare_dependency_versions.
type CompareDependencyVersionsArgs struct {
	Dependencies    []DependencySpec       `json:"dependencies" jsonschema:"required,List of dependencies to compare with their current versions"`
	StabilityFilter domain.StabilityFilter `json:"stabilityFilter" jsonschema:"The stability filter: ALL, STABLE_ONLY, or PREFER_STABLE (default: PREFER_STABLE)"`
}

// PingArgs represents arguments for ping.
type PingArgs struct {
	Message string `json:"message" jsonschema:"Optional message to echo back"`
}

// handleGetLatestVersion handles the get_latest_version tool.
func (s *Server) handleGetLatestVersion(ctx context.Context, req *mcp.CallToolRequest, args GetLatestVersionArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleGetLatestVersion called", "groupId", args.GroupID, "artifactId", args.ArtifactID)

	// Set default stability filter
	if args.StabilityFilter == "" {
		args.StabilityFilter = domain.StabilityPreferStable
	}

	// Parse coordinates
	coord := &domain.Coordinates{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Packaging:  "jar",
	}

	// Validate
	if err := coord.Validate(); err != nil {
		return errorResult("INVALID_COORDINATES", err.Error()), nil, nil
	}

	// Get latest version
	latest, err := s.client.GetLatestVersion(ctx, coord, args.StabilityFilter)
	if err != nil {
		if mavencentral.IsNotFound(err) {
			return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
		}
		observability.Error("failed to get latest version", "error", err)
		return errorResult("UPSTREAM_UNAVAILABLE", err.Error()), nil, nil
	}

	if latest == "" {
		return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
	}

	// Get version type
	vc := domain.NewVersionComparator()
	versionType := vc.GetVersionType(latest)

	// Build result
	result := map[string]any{
		"coordinate":      fmt.Sprintf("%s:%s:%s", coord.GroupID, coord.ArtifactID, latest),
		"groupId":         coord.GroupID,
		"artifactId":      coord.ArtifactID,
		"latestVersion":   latest,
		"stability":       versionType.String(),
		"isStable":        versionType.IsStable(),
		"stabilityFilter": string(args.StabilityFilter),
	}

	return successResult(result), nil, nil
}

// handleCheckVersionExists handles the check_version_exists tool.
func (s *Server) handleCheckVersionExists(ctx context.Context, req *mcp.CallToolRequest, args CheckVersionExistsArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleCheckVersionExists called", "groupId", args.GroupID, "artifactId", args.ArtifactID, "version", args.Version)

	// Parse coordinates
	coord := &domain.Coordinates{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Version:    args.Version,
		Packaging:  "jar",
	}

	// Validate
	if err := coord.Validate(); err != nil {
		return errorResult("INVALID_COORDINATES", err.Error()), nil, nil
	}

	// Check if version exists
	exists, err := s.client.VersionExists(ctx, coord, args.Version)
	if err != nil {
		observability.Error("failed to check version", "error", err)
		// Return false rather than error for this tool
	}

	// Get version type for additional info
	vc := domain.NewVersionComparator()
	versionType := vc.GetVersionType(args.Version)

	// Build result
	result := map[string]any{
		"coordinate": coord.String(),
		"groupId":    coord.GroupID,
		"artifactId": coord.ArtifactID,
		"version":    args.Version,
		"exists":     exists,
		"type":       versionType.String(),
		"isStable":   versionType.IsStable(),
	}

	return successResult(result), nil, nil
}

// handlePing handles the ping tool.
func (s *Server) handlePing(ctx context.Context, req *mcp.CallToolRequest, args PingArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handlePing called", "message", args.Message)

	result := map[string]any{
		"message":   "pong",
		"echo":      args.Message,
		"server":    s.config.ServerName,
		"version":   s.config.ServerVersion,
		"transport": string(s.config.Transport),
	}

	return successResult(result), nil, nil
}

// handleCheckMultipleDependencies handles the check_multiple_dependencies tool.
func (s *Server) handleCheckMultipleDependencies(ctx context.Context, req *mcp.CallToolRequest, args CheckMultipleDependenciesArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleCheckMultipleDependencies called", "count", len(args.Dependencies))

	if result := s.validateDependencyBatch(args.Dependencies); result != nil {
		return result, nil, nil
	}

	vc := domain.NewVersionComparator()
	results := make([]map[string]any, len(args.Dependencies))

	var wg sync.WaitGroup
	sem := make(chan struct{}, s.concurrencyLimit(len(args.Dependencies)))
	for i, dep := range args.Dependencies {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, d DependencySpec) {
			defer wg.Done()
			defer func() { <-sem }()

			coord := &domain.Coordinates{
				GroupID:    d.GroupID,
				ArtifactID: d.ArtifactID,
				Packaging:  "jar",
			}

			info := map[string]any{
				"groupId":    d.GroupID,
				"artifactId": d.ArtifactID,
				"status":     "success",
				"exists":     false,
			}

			// Check if version is specified
			if d.Version != "" {
				info["version"] = d.Version
				exists, _ := s.client.VersionExists(ctx, coord, d.Version)
				info["exists"] = exists
				info["type"] = vc.GetVersionType(d.Version).String()
				info["isStable"] = vc.IsStable(d.Version)
			} else {
				// Just check if artifact exists
				versions, err := s.client.GetAllVersions(ctx, coord)
				if err != nil || len(versions) == 0 {
					info["status"] = "not_found"
					info["exists"] = false
				} else {
					info["exists"] = true
					info["latestVersion"] = versions[0]
				}
			}

			results[idx] = info
		}(i, dep)
	}

	wg.Wait()

	result := map[string]any{
		"count":        len(args.Dependencies),
		"dependencies": results,
	}

	return successResult(result), nil, nil
}

// handleCompareDependencyVersions handles the compare_dependency_versions tool.
func (s *Server) handleCompareDependencyVersions(ctx context.Context, req *mcp.CallToolRequest, args CompareDependencyVersionsArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleCompareDependencyVersions called", "count", len(args.Dependencies))

	if result := s.validateDependencyBatch(args.Dependencies); result != nil {
		return result, nil, nil
	}

	// Set default stability filter
	if args.StabilityFilter == "" {
		args.StabilityFilter = domain.StabilityPreferStable
	}

	vc := domain.NewVersionComparator()
	results := make([]map[string]any, len(args.Dependencies))
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.concurrencyLimit(len(args.Dependencies)))

	for i, dep := range args.Dependencies {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, d DependencySpec) {
			defer wg.Done()
			defer func() { <-sem }()

			coord := &domain.Coordinates{
				GroupID:    d.GroupID,
				ArtifactID: d.ArtifactID,
				Packaging:  "jar",
			}

			info := map[string]any{
				"groupId":    d.GroupID,
				"artifactId": d.ArtifactID,
				"status":     "success",
			}

			// Get current version if provided
			if d.Version != "" {
				info["currentVersion"] = d.Version
				info["currentType"] = vc.GetVersionType(d.Version).String()
				info["currentIsStable"] = vc.IsStable(d.Version)
			}

			// Get latest version
			latest, err := s.client.GetLatestVersion(ctx, coord, args.StabilityFilter)
			if err != nil || latest == "" {
				info["status"] = "not_found"
				info["exists"] = false
			} else {
				info["latestVersion"] = latest
				info["latestType"] = vc.GetVersionType(latest).String()
				info["latestIsStable"] = vc.IsStable(latest)
				info["exists"] = true

				// Compare versions if current is provided
				if d.Version != "" {
					updateType := vc.DetermineUpdateType(d.Version, latest)
					info["updateType"] = updateType.String()
					info["updateAvailable"] = updateType != domain.UpdateNone
					if fallback := s.buildSameMajorStableFallback(ctx, coord, d.Version, latest, args.StabilityFilter); fallback != nil {
						info["sameMajorStableFallback"] = fallback
					}
				}
			}

			results[idx] = info
		}(i, dep)
	}

	wg.Wait()

	// Count updates available
	updatesAvailable := 0
	for _, r := range results {
		if ua, ok := r["updateAvailable"].(bool); ok && ua {
			updatesAvailable++
		}
	}

	result := map[string]any{
		"count":            len(args.Dependencies),
		"updatesAvailable": updatesAvailable,
		"stabilityFilter":  string(args.StabilityFilter),
		"dependencies":     results,
	}

	return successResult(result), nil, nil
}

func (s *Server) validateDependencyBatch(dependencies []DependencySpec) *mcp.CallToolResult {
	if len(dependencies) == 0 {
		return errorResult("INVALID_ARGUMENTS", "At least one dependency must be provided")
	}
	if len(dependencies) > s.config.MaxDependencies {
		return errorResult("INVALID_ARGUMENTS", fmt.Sprintf("At most %d dependencies may be provided", s.config.MaxDependencies))
	}
	for i, dep := range dependencies {
		coord := &domain.Coordinates{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Packaging:  "jar",
		}
		if err := coord.Validate(); err != nil {
			return errorResult("INVALID_COORDINATES", fmt.Sprintf("dependency at index %d: %s", i, err.Error()))
		}
	}
	return nil
}

func (s *Server) concurrencyLimit(total int) int {
	limit := s.config.MaxConcurrentRequests
	if limit <= 0 {
		limit = 1
	}
	if total > 0 && limit > total {
		return total
	}
	return limit
}

func (s *Server) buildSameMajorStableFallback(ctx context.Context, coord *domain.Coordinates, currentVersion, latestVersion string, filter domain.StabilityFilter) map[string]any {
	if filter != domain.StabilityStableOnly {
		return nil
	}

	vc := domain.NewVersionComparator()
	if vc.DetermineUpdateType(currentVersion, latestVersion) != domain.UpdateMajor {
		return nil
	}

	currentParts := vc.ParseVersion(currentVersion).NumericParts
	if len(currentParts) == 0 {
		return nil
	}
	currentMajor := currentParts[0]

	versions, err := s.client.GetAllVersions(ctx, coord)
	if err != nil {
		return nil
	}

	for _, candidate := range versions {
		if candidate == currentVersion {
			break
		}
		if !vc.IsStable(candidate) {
			continue
		}

		candidateParts := vc.ParseVersion(candidate).NumericParts
		if len(candidateParts) == 0 || candidateParts[0] != currentMajor {
			continue
		}
		if vc.Compare(currentVersion, candidate) >= 0 {
			continue
		}

		updateType := vc.DetermineUpdateType(currentVersion, candidate)
		if updateType != domain.UpdateMinor && updateType != domain.UpdatePatch {
			continue
		}

		return map[string]any{
			"latestVersion": candidate,
			"updateType":    updateType.String(),
		}
	}

	return nil
}

// successResult creates a successful tool result with JSON content.
func successResult(data any) *mcp.CallToolResult {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return errorResult("INTERNAL_ERROR", fmt.Sprintf("failed to encode tool response: %v", err))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}
}

// errorResult creates an error tool result.
func errorResult(errorType, message string) *mcp.CallToolResult {
	data := map[string]any{
		"status":    "error",
		"errorType": errorType,
		"message":   message,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		jsonData = []byte(`{"status":"error","errorType":"INTERNAL_ERROR","message":"failed to encode error response"}`)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
		IsError: true,
	}
}
