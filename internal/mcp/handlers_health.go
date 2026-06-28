package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AnalyzeProjectHealthArgs represents arguments for analyze_project_health.
type AnalyzeProjectHealthArgs struct {
	Dependencies    []DependencySpec       `json:"dependencies" jsonschema:"required,List of dependencies to analyze"`
	IncludeLicenses bool                   `json:"includeLicenses,omitempty" jsonschema:"Whether to include license information"`
	StabilityFilter domain.StabilityFilter `json:"stabilityFilter,omitempty" jsonschema:"The stability filter for latest version checks"`
}

// handleAnalyzeProjectHealth handles the analyze_project_health tool.
func (s *Server) handleAnalyzeProjectHealth(ctx context.Context, req *mcp.CallToolRequest, args AnalyzeProjectHealthArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleAnalyzeProjectHealth called", "count", len(args.Dependencies))

	if result := s.validateDependencyBatch(args.Dependencies); result != nil {
		return result, nil, nil
	}

	// Set default stability filter
	if args.StabilityFilter == "" {
		args.StabilityFilter = domain.StabilityPreferStable
	}

	now := time.Now()
	vc := domain.NewVersionComparator()

	ageDistribution := map[string]int{"fresh": 0, "current": 0, "aging": 0, "stale": 0}
	analyses := make([]map[string]any, len(args.Dependencies))
	recommendations := []string{}

	var wg sync.WaitGroup
	var mu sync.Mutex
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

			analysis := map[string]any{
				"groupId":    d.GroupID,
				"artifactId": d.ArtifactID,
				"status":     "success",
			}

			latestVersion, err := s.client.GetLatestVersion(ctx, coord, args.StabilityFilter)
			if err != nil || latestVersion == "" {
				analysis["status"] = "not_found"
				analysis["error"] = "Dependency not found in Maven Central"
				mu.Lock()
				analyses[idx] = analysis
				mu.Unlock()
				return
			}

			latest, err := s.client.GetArtifactWithTimestamp(ctx, coord, latestVersion)
			if err != nil {
				analysis["status"] = "not_found"
				analysis["error"] = "Dependency not found in Maven Central"
				mu.Lock()
				analyses[idx] = analysis
				mu.Unlock()
				return
			}

			daysSinceRelease := (now.UnixMilli() - latest.Timestamp) / (24 * 60 * 60 * 1000)
			freshness := domain.ClassifyFreshness(int(daysSinceRelease))

			analysis["latestVersion"] = latest.Version
			analysis["daysSinceRelease"] = daysSinceRelease
			analysis["ageClassification"] = freshness.String()
			analysis["stability"] = vc.GetVersionType(latest.Version).String()
			analysis["isStable"] = vc.IsStable(latest.Version)

			// Calculate health score (0-100)
			healthScore := calculateHealthScore(int(daysSinceRelease), freshness, vc.IsStable(latest.Version))
			analysis["healthScore"] = healthScore

			// Determine maintenance level
			analysis["maintenanceLevel"] = determineMaintenanceLevel(int(daysSinceRelease), 1)

			// Check for updates if current version provided
			if d.Version != "" {
				analysis["currentVersion"] = d.Version
				updateType := vc.DetermineUpdateType(d.Version, latest.Version)
				analysis["updateType"] = updateType.String()
				analysis["updateAvailable"] = updateType != domain.UpdateNone

				if updateType == domain.UpdateMajor {
					mu.Lock()
					recommendations = append(recommendations, fmt.Sprintf("%s:%s has a major update available (%s -> %s)", d.GroupID, d.ArtifactID, d.Version, latest.Version))
					mu.Unlock()
				}
			}

			// Include licenses if requested
			if args.IncludeLicenses {
				coord.Version = latest.Version
				licenses, _ := s.client.GetLicenses(ctx, coord)
				if len(licenses) > 0 {
					licenseInfos := make([]map[string]string, len(licenses))
					for i, lic := range licenses {
						licenseInfos[i] = map[string]string{
							"name": lic.Name,
							"url":  lic.URL,
						}
					}
					analysis["licenses"] = licenseInfos
				}
			}

			mu.Lock()
			ageDistribution[freshness.String()]++
			analyses[idx] = analysis
			mu.Unlock()
		}(i, dep)
	}

	wg.Wait()

	// Generate aggregate recommendations
	if len(recommendations) == 0 {
		if ageDistribution["stale"] > 0 {
			recommendations = append(recommendations, fmt.Sprintf("%d dependencies are stale (>365 days old) - consider updating", ageDistribution["stale"]))
		}
		if ageDistribution["aging"] > 0 {
			recommendations = append(recommendations, fmt.Sprintf("%d dependencies are aging (90-365 days old) - monitor for updates", ageDistribution["aging"]))
		}
		if ageDistribution["fresh"] > 0 {
			recommendations = append(recommendations, fmt.Sprintf("%d dependencies are fresh - good job keeping up to date!", ageDistribution["fresh"]))
		}
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "All dependencies appear to be in good health")
	}

	// Calculate success/failure counts
	successful := 0
	failed := 0
	for _, a := range analyses {
		if a["status"] == "success" {
			successful++
		} else {
			failed++
		}
	}

	result := map[string]any{
		"analysisDate":       now.Format(time.RFC3339),
		"dependencyCount":    len(args.Dependencies),
		"successfulAnalysis": successful,
		"failedAnalysis":     failed,
		"ageDistribution":    ageDistribution,
		"dependencies":       analyses,
		"recommendations":    recommendations,
	}

	return successResult(result), nil, nil
}

// calculateHealthScore calculates a health score (0-100) for a dependency.
func calculateHealthScore(daysSinceRelease int, freshness domain.Freshness, isStable bool) int {
	score := 100

	// Deduct points based on age
	switch freshness {
	case domain.FreshnessFresh:
		// Full score for fresh dependencies
	case domain.FreshnessCurrent:
		score -= 10
	case domain.FreshnessAging:
		score -= 30
	case domain.FreshnessStale:
		score -= 60
	}

	// Deduct points for unstable versions
	if !isStable {
		score -= 10
	}

	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// determineMaintenanceLevel determines the maintenance level of a dependency.
func determineMaintenanceLevel(daysSinceRelease int, totalVersions int) string {
	if daysSinceRelease < 30 {
		return "active"
	} else if daysSinceRelease < 180 {
		if totalVersions > 10 {
			return "mature"
		}
		return "maintained"
	} else if daysSinceRelease < 365 {
		return "inactive"
	}
	return "abandoned"
}
