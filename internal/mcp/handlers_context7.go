package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ctx7 "github.com/aparcero/mcp-maven/internal/context7"
	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetLibraryDocsArgs represents arguments for get_library_docs.
type GetLibraryDocsArgs struct {
	GroupID    string `json:"groupId" jsonschema:"required,The Maven groupId"`
	ArtifactID string `json:"artifactId" jsonschema:"required,The Maven artifactId"`
	Version    string `json:"version,omitempty" jsonschema:"Optional version (defaults to latest)"`
	Query      string `json:"query,omitempty" jsonschema:"Optional specific query for documentation"`
}

// handleGetLibraryDocs handles the get_library_docs tool. It resolves a Maven
// coordinate to a Context7 library, fetches matching documentation, and returns
// it together with curated reference links.
func (s *Server) handleGetLibraryDocs(ctx context.Context, req *mcp.CallToolRequest, args GetLibraryDocsArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleGetLibraryDocs called", "groupId", args.GroupID, "artifactId", args.ArtifactID)

	if !s.docs.Enabled() {
		return errorResult("SERVICE_UNAVAILABLE", "Context7 documentation provider is not enabled. Set CONTEXT7_ENABLED=true to enable."), nil, nil
	}

	coord := &domain.Coordinates{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Version:    args.Version,
		Packaging:  "jar",
	}
	if err := coord.Validate(); err != nil {
		return errorResult("INVALID_COORDINATES", err.Error()), nil, nil
	}

	if args.Version == "" {
		latest, err := s.client.GetLatestVersion(ctx, coord, domain.StabilityPreferStable)
		if err != nil {
			observability.Error("failed to get latest version", "error", err)
			return errorResult("UPSTREAM_UNAVAILABLE", err.Error()), nil, nil
		}
		if latest == "" {
			return errorResult("NOT_FOUND", "No versions found for this artifact"), nil, nil
		}
		args.Version = latest
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		query = args.ArtifactID + " getting started overview"
	}

	docs, err := s.docs.GetLibraryDocs(ctx, ctx7.DocsQuery{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Version:    args.Version,
		Query:      query,
	})
	if err != nil {
		return context7ErrorResult(err), nil, nil
	}

	result := map[string]any{
		"groupId":      args.GroupID,
		"artifactId":   args.ArtifactID,
		"version":      args.Version,
		"status":       "success",
		"libraryId":    docs.LibraryID,
		"query":        docs.Query,
		"matches":      docs.Matches,
		"codeSnippets": docs.CodeSnippets,
		"infoSnippets": docs.InfoSnippets,
		"documentationUrl": map[string]string{
			"javadoc":       fmt.Sprintf("https://javadoc.io/doc/%s/%s/%s/", args.GroupID, args.ArtifactID, args.Version),
			"github":        "https://github.com/search?q=" + args.GroupID + "+" + args.ArtifactID + "+docs",
			"mvnrepository": fmt.Sprintf("https://mvnrepository.com/artifact/%s/%s/%s", args.GroupID, args.ArtifactID, args.Version),
		},
	}

	return successResult(result), nil, nil
}

// context7ErrorResult maps a Context7 provider error to a business error result.
func context7ErrorResult(err error) *mcp.CallToolResult {
	var apiErr *ctx7.APIError
	if ctx7.AsAPIError(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusAccepted:
			return errorResult("PENDING", apiErr.Error())
		case http.StatusMovedPermanently:
			return errorResult("REDIRECT", apiErr.Error())
		case http.StatusBadRequest:
			return errorResult("INVALID_ARGUMENTS", apiErr.Error())
		case http.StatusUnauthorized, http.StatusForbidden:
			return errorResult("UNAUTHORIZED", apiErr.Error())
		case http.StatusNotFound:
			return errorResult("NOT_FOUND", apiErr.Error())
		case http.StatusTooManyRequests:
			return errorResult("RATE_LIMITED", apiErr.Error())
		default:
			if apiErr.StatusCode >= 500 {
				return errorResult("UPSTREAM_UNAVAILABLE", apiErr.Error())
			}
		}
	}

	return errorResult("UPSTREAM_UNAVAILABLE", err.Error())
}
