package context7

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
)

const defaultBaseURL = "https://context7.com"

// DocsProvider provides Context7-backed documentation resolution.
type DocsProvider interface {
	ResolveLibraryID(ctx context.Context, query ResolveLibraryIDQuery) (*ResolveLibraryIDResult, error)
	QueryDocs(ctx context.Context, query QueryDocsQuery) (*QueryDocsResult, error)
	GetLibraryDocs(ctx context.Context, query DocsQuery) (*DocsResult, error)
	Enabled() bool
}

// ResolveLibraryIDQuery searches for a library by name.
type ResolveLibraryIDQuery struct {
	LibraryName string
	Query       string
}

// QueryDocsQuery fetches documentation for a specific Context7 library.
type QueryDocsQuery struct {
	LibraryID string
	Query     string
}

// DocsQuery resolves then fetches docs, used by the compatibility wrapper tool.
type DocsQuery struct {
	GroupID    string
	ArtifactID string
	Version    string
	Query      string
}

// LibraryMatch is a normalized library search result.
type LibraryMatch struct {
	LibraryID        string   `json:"libraryId"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	CodeSnippets     int      `json:"codeSnippets"`
	SourceReputation string   `json:"sourceReputation"`
	BenchmarkScore   float64  `json:"benchmarkScore"`
	Versions         []string `json:"versions"`
	TrustScore       float64  `json:"trustScore,omitempty"`
	State            string   `json:"state,omitempty"`
	LastUpdateDate   string   `json:"lastUpdateDate,omitempty"`
}

// ResolveLibraryIDResult is the normalized search response.
type ResolveLibraryIDResult struct {
	Results             []LibraryMatch `json:"results"`
	SearchFilterApplied bool           `json:"searchFilterApplied,omitempty"`
}

// CodeBlock is a single code example block.
type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// CodeSnippet is a structured documentation code result.
type CodeSnippet struct {
	CodeTitle       string      `json:"codeTitle"`
	CodeDescription string      `json:"codeDescription"`
	CodeLanguage    string      `json:"codeLanguage"`
	CodeTokens      int         `json:"codeTokens"`
	CodeID          string      `json:"codeId"`
	PageTitle       string      `json:"pageTitle"`
	CodeList        []CodeBlock `json:"codeList"`
}

// InfoSnippet is a prose documentation result.
type InfoSnippet struct {
	PageID        string `json:"pageId"`
	Breadcrumb    string `json:"breadcrumb"`
	Content       string `json:"content"`
	ContentTokens int    `json:"contentTokens"`
}

// QueryDocsResult is the normalized docs response.
type QueryDocsResult struct {
	LibraryID    string        `json:"libraryId"`
	Query        string        `json:"query"`
	CodeSnippets []CodeSnippet `json:"codeSnippets"`
	InfoSnippets []InfoSnippet `json:"infoSnippets"`
}

// DocsResult powers the get_library_docs compatibility wrapper.
type DocsResult struct {
	LibraryID    string         `json:"libraryId"`
	Query        string         `json:"query"`
	Matches      []LibraryMatch `json:"matches"`
	CodeSnippets []CodeSnippet  `json:"codeSnippets"`
	InfoSnippets []InfoSnippet  `json:"infoSnippets"`
}

// APIError represents a Context7 HTTP/API failure.
type APIError struct {
	StatusCode  int
	Code        string
	Message     string
	RedirectURL string
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("Context7 API error (%d)", e.StatusCode)
}

// Client implements the Context7 docs provider using the HTTP API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	enabled    bool
}

// NewClient creates a new Context7 HTTP client.
func NewClient(cfg config.Context7Config) DocsProvider {
	if !cfg.Enabled {
		return NewNoopProvider()
	}

	baseURL := strings.TrimRight(cfg.ServerURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(cfg.APIKey),
		enabled:    true,
	}
}

// NewNoopProvider creates a no-op provider.
func NewNoopProvider() DocsProvider {
	return &noopProvider{}
}

type noopProvider struct{}

func (n *noopProvider) ResolveLibraryID(ctx context.Context, query ResolveLibraryIDQuery) (*ResolveLibraryIDResult, error) {
	return nil, fmt.Errorf("documentation provider not enabled")
}

func (n *noopProvider) QueryDocs(ctx context.Context, query QueryDocsQuery) (*QueryDocsResult, error) {
	return nil, fmt.Errorf("documentation provider not enabled")
}

func (n *noopProvider) GetLibraryDocs(ctx context.Context, query DocsQuery) (*DocsResult, error) {
	return nil, fmt.Errorf("documentation provider not enabled")
}

func (n *noopProvider) Enabled() bool {
	return false
}

func (c *Client) Enabled() bool {
	return c.enabled
}

// ResolveLibraryID searches Context7 for matching libraries.
func (c *Client) ResolveLibraryID(ctx context.Context, query ResolveLibraryIDQuery) (*ResolveLibraryIDResult, error) {
	if strings.TrimSpace(query.LibraryName) == "" {
		return nil, fmt.Errorf("libraryName is required")
	}
	if strings.TrimSpace(query.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	var payload struct {
		Results []struct {
			ID             string   `json:"id"`
			Title          string   `json:"title"`
			Description    string   `json:"description"`
			TotalSnippets  int      `json:"totalSnippets"`
			TrustScore     float64  `json:"trustScore"`
			BenchmarkScore float64  `json:"benchmarkScore"`
			Versions       []string `json:"versions"`
			State          string   `json:"state"`
			LastUpdateDate string   `json:"lastUpdateDate"`
		} `json:"results"`
		SearchFilterApplied bool `json:"searchFilterApplied"`
	}

	params := url.Values{}
	params.Set("libraryName", query.LibraryName)
	params.Set("query", query.Query)
	if err := c.getJSON(ctx, "/api/v2/libs/search", params, &payload); err != nil {
		return nil, err
	}

	results := make([]LibraryMatch, 0, len(payload.Results))
	for _, item := range payload.Results {
		results = append(results, LibraryMatch{
			LibraryID:        item.ID,
			Name:             item.Title,
			Description:      item.Description,
			CodeSnippets:     item.TotalSnippets,
			SourceReputation: classifySourceReputation(item.TrustScore),
			BenchmarkScore:   item.BenchmarkScore,
			Versions:         item.Versions,
			TrustScore:       item.TrustScore,
			State:            item.State,
			LastUpdateDate:   item.LastUpdateDate,
		})
	}

	return &ResolveLibraryIDResult{
		Results:             results,
		SearchFilterApplied: payload.SearchFilterApplied,
	}, nil
}

// QueryDocs fetches documentation snippets for a specific library id.
func (c *Client) QueryDocs(ctx context.Context, query QueryDocsQuery) (*QueryDocsResult, error) {
	return c.queryDocs(ctx, query, 0)
}

func (c *Client) queryDocs(ctx context.Context, query QueryDocsQuery, redirects int) (*QueryDocsResult, error) {
	if strings.TrimSpace(query.LibraryID) == "" {
		return nil, fmt.Errorf("libraryId is required")
	}
	if strings.TrimSpace(query.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if redirects > 3 {
		return nil, fmt.Errorf("too many context7 redirects")
	}

	var payload struct {
		CodeSnippets []CodeSnippet `json:"codeSnippets"`
		InfoSnippets []InfoSnippet `json:"infoSnippets"`
	}

	params := url.Values{}
	params.Set("libraryId", query.LibraryID)
	params.Set("query", query.Query)
	params.Set("type", "json")
	err := c.getJSON(ctx, "/api/v2/context", params, &payload)
	if err == nil {
		return &QueryDocsResult{
			LibraryID:    query.LibraryID,
			Query:        query.Query,
			CodeSnippets: payload.CodeSnippets,
			InfoSnippets: payload.InfoSnippets,
		}, nil
	}

	var apiErr *APIError
	if !AsAPIError(err, &apiErr) || apiErr.StatusCode != http.StatusMovedPermanently || apiErr.RedirectURL == "" {
		return nil, err
	}

	redirectedID := libraryIDFromRedirect(apiErr.RedirectURL)
	if redirectedID == "" {
		return nil, err
	}
	return c.queryDocs(ctx, QueryDocsQuery{LibraryID: redirectedID, Query: query.Query}, redirects+1)
}

// GetLibraryDocs resolves a library and then queries docs, preserving compatibility
// with the older convenience tool shape used by the Go server.
func (c *Client) GetLibraryDocs(ctx context.Context, query DocsQuery) (*DocsResult, error) {
	libraryName := strings.TrimSpace(query.ArtifactID)
	if libraryName == "" {
		return nil, fmt.Errorf("artifactId is required")
	}

	searchQuery := strings.TrimSpace(query.Query)
	if searchQuery == "" {
		searchQuery = libraryName + " getting started overview"
	}

	resolved, err := c.ResolveLibraryID(ctx, ResolveLibraryIDQuery{
		LibraryName: libraryName,
		Query:       searchQuery,
	})
	if err != nil {
		return nil, err
	}
	if len(resolved.Results) == 0 {
		return nil, &APIError{StatusCode: http.StatusNotFound, Code: "library_not_found", Message: fmt.Sprintf("no Context7 library match found for %s", libraryName)}
	}

	selected := resolved.Results[0]
	libraryID := resolveVersionedLibraryID(selected, query.Version)
	docs, err := c.QueryDocs(ctx, QueryDocsQuery{
		LibraryID: libraryID,
		Query:     searchQuery,
	})
	if err != nil {
		return nil, err
	}

	return &DocsResult{
		LibraryID:    docs.LibraryID,
		Query:        docs.Query,
		Matches:      resolved.Results,
		CodeSnippets: docs.CodeSnippets,
		InfoSnippets: docs.InfoSnippets,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, path string, params url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Context7-API-Key", c.apiKey)
		req.Header.Set("X-Context7-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode Context7 response: %w", err)
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var payload struct {
		Error       string `json:"error"`
		Message     string `json:"message"`
		RedirectURL string `json:"redirectUrl"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	retryAfter := time.Duration(0)
	if value := resp.Header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			retryAfter = time.Duration(seconds) * time.Second
		}
	}

	return &APIError{
		StatusCode:  resp.StatusCode,
		Code:        payload.Error,
		Message:     payload.Message,
		RedirectURL: payload.RedirectURL,
		RetryAfter:  retryAfter,
	}
}

// AsAPIError unwraps a Context7 API error.
func AsAPIError(err error, target **APIError) bool {
	apiErr, ok := err.(*APIError)
	if !ok {
		return false
	}
	*target = apiErr
	return true
}

func classifySourceReputation(score float64) string {
	switch {
	case score >= 8:
		return "High"
	case score >= 5:
		return "Medium"
	case score > 0:
		return "Low"
	default:
		return "Unknown"
	}
}

func resolveVersionedLibraryID(match LibraryMatch, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return match.LibraryID
	}

	for _, candidate := range match.Versions {
		if normalizeVersion(candidate) == normalizeVersion(version) {
			return strings.TrimRight(match.LibraryID, "/") + "/" + candidate
		}
	}
	return match.LibraryID
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(strings.ToLower(version))
	version = strings.TrimPrefix(version, "v")
	version = strings.ReplaceAll(version, "_", ".")
	return version
}

func libraryIDFromRedirect(redirect string) string {
	if strings.HasPrefix(redirect, "/") {
		return redirect
	}

	u, err := url.Parse(redirect)
	if err != nil {
		return ""
	}
	if libraryID := u.Query().Get("libraryId"); libraryID != "" {
		return libraryID
	}
	if strings.HasPrefix(u.Path, "/") {
		return u.Path
	}
	return ""
}
