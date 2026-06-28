package mcp

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/mavencentral"
	"github.com/aparcero/mcp-maven/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetVersionTimelineArgs represents arguments for get_version_timeline.
type GetVersionTimelineArgs struct {
	GroupID    string `json:"groupId" jsonschema:"required,The Maven groupId"`
	ArtifactID string `json:"artifactId" jsonschema:"required,The Maven artifactId"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of versions to include (default: 30)"`
}

// AnalyzeDependencyAgeArgs represents arguments for analyze_dependency_age.
type AnalyzeDependencyAgeArgs struct {
	Dependencies []DependencySpec `json:"dependencies" jsonschema:"required,List of dependencies to analyze"`
}

// AnalyzeReleasePatternsArgs represents arguments for analyze_release_patterns.
type AnalyzeReleasePatternsArgs struct {
	GroupID    string `json:"groupId" jsonschema:"required,The Maven groupId"`
	ArtifactID string `json:"artifactId" jsonschema:"required,The Maven artifactId"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of versions to analyze (default: 30)"`
}

// handleGetVersionTimeline handles the get_version_timeline tool.
func (s *Server) handleGetVersionTimeline(ctx context.Context, req *mcp.CallToolRequest, args GetVersionTimelineArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleGetVersionTimeline called", "groupId", args.GroupID, "artifactId", args.ArtifactID)

	// Set default limit
	if args.Limit <= 0 {
		args.Limit = 30
	}

	coord := &domain.Coordinates{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Packaging:  "jar",
	}

	if err := coord.Validate(); err != nil {
		return errorResult("INVALID_COORDINATES", err.Error()), nil, nil
	}

	// Get versions with timestamps
	artifacts, err := s.client.GetVersionsWithTimestamps(ctx, coord, args.Limit)
	if err != nil {
		observability.Error("failed to get versions with timestamps", "error", err)
		if mavencentral.IsNotFound(err) {
			return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
		}
		return errorResult("UPSTREAM_UNAVAILABLE", err.Error()), nil, nil
	}

	if len(artifacts) == 0 {
		return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
	}

	// Build timeline
	vc := domain.NewVersionComparator()
	now := time.Now()
	timeline := make([]map[string]any, len(artifacts))
	totalDays := int(now.Sub(time.UnixMilli(artifacts[len(artifacts)-1].Timestamp)).Hours() / 24)

	var sumDaysSincePrevious float64
	var gapCount int
	releaseGaps := make(map[string]int)

	for i, artifact := range artifacts {
		var daysSincePrevious int64
		var gap domain.ReleaseGap
		var breakingChange bool

		if i < len(artifacts)-1 {
			prevTimestamp := artifacts[i+1].Timestamp
			daysSincePrevious = (artifact.Timestamp - prevTimestamp) / (24 * 60 * 60 * 1000)
			breakingChange = isBreakingChange(artifact.Version, artifacts[i+1].Version)
		}

		// Calculate gap if we have data
		if i < len(artifacts)-1 && daysSincePrevious > 0 {
			sumDaysSincePrevious += float64(daysSincePrevious)
			gapCount++
			avgInterval := sumDaysSincePrevious / float64(gapCount)
			gap = domain.ClassifyReleaseGap(int(daysSincePrevious), avgInterval)
		} else {
			gap = domain.ReleaseGapNormal
		}

		releaseGaps[string(gap)]++

		timeline[i] = map[string]any{
			"version":           artifact.Version,
			"versionType":       vc.GetVersionType(artifact.Version).String(),
			"releaseDate":       artifact.Timestamp,
			"relativeTime":      domain.FormatRelativeTime(time.UnixMilli(artifact.Timestamp)),
			"daysSincePrevious": daysSincePrevious,
			"isBreakingChange":  breakingChange,
			"releaseGap":        string(gap),
		}
	}

	// Calculate velocity trend
	avgInterval := 0.0
	if gapCount > 0 {
		avgInterval = sumDaysSincePrevious / float64(gapCount)
	}
	recentVelocity := 0.0
	if avgInterval > 0 {
		recentVelocity = 30.0 / avgInterval // releases per month
	}

	velocityTrend := map[string]any{
		"trend":              "stable",
		"description":        "Stable release cadence",
		"recentVelocity":     recentVelocity,
		"historicalVelocity": recentVelocity,
		"changePercentage":   0.0,
	}

	// Calculate stability pattern
	stableCount := 0
	for _, artifact := range artifacts {
		if vc.IsStable(artifact.Version) {
			stableCount++
		}
	}
	stablePct := float64(stableCount) / float64(len(artifacts)) * 100

	stabilityPattern := map[string]any{
		"stablePercentage":     stablePct,
		"prereleasePattern":    "standard",
		"stableReleasePattern": "regular",
		"recommendation":       "Stable release pattern",
	}

	// Recent activity
	nowMs := now.UnixMilli()
	lastReleaseAge := (nowMs - artifacts[0].Timestamp) / (24 * 60 * 60 * 1000)
	recentActivity := map[string]any{
		"releasesLastMonth":   countReleasesInPeriod(artifacts, now, 30),
		"releasesLastQuarter": countReleasesInPeriod(artifacts, now, 90),
		"lastReleaseAge":      lastReleaseAge,
		"activityLevel":       domain.ClassifyActivityLevel(countReleasesInPeriod(artifacts, now, 30), countReleasesInPeriod(artifacts, now, 90)).String(),
	}

	// Generate insights
	insights := generateInsights(artifacts, stablePct, lastReleaseAge, releaseGaps)

	result := map[string]any{
		"dependency":           coord.String(),
		"totalVersions":        len(artifacts),
		"versionsReturned":     len(artifacts),
		"timeSpanMonths":       totalDays / 30,
		"versionTimeline":      timeline,
		"releaseVelocityTrend": velocityTrend,
		"stabilityPattern":     stabilityPattern,
		"recentActivity":       recentActivity,
		"insights":             insights,
	}

	return successResult(result), nil, nil
}

// handleAnalyzeDependencyAge handles the analyze_dependency_age tool.
func (s *Server) handleAnalyzeDependencyAge(ctx context.Context, req *mcp.CallToolRequest, args AnalyzeDependencyAgeArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleAnalyzeDependencyAge called", "count", len(args.Dependencies))

	if result := s.validateDependencyBatch(args.Dependencies); result != nil {
		return result, nil, nil
	}

	now := time.Now()
	vc := domain.NewVersionComparator()
	ageDistribution := map[string]int{"fresh": 0, "current": 0, "aging": 0, "stale": 0}
	analyses := make([]map[string]any, len(args.Dependencies))

	for i, dep := range args.Dependencies {
		coord := &domain.Coordinates{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Packaging:  "jar",
		}

		analysis := map[string]any{
			"groupId":    dep.GroupID,
			"artifactId": dep.ArtifactID,
			"status":     "success",
		}

		// Get latest version with timestamp
		artifacts, err := s.client.GetVersionsWithTimestamps(ctx, coord, 1)
		if err != nil || len(artifacts) == 0 {
			analysis["status"] = "not_found"
			analysis["error"] = "Dependency not found"
			analyses[i] = analysis
			continue
		}

		latest := artifacts[0]
		daysSinceRelease := (now.UnixMilli() - latest.Timestamp) / (24 * 60 * 60 * 1000)
		freshness := domain.ClassifyFreshness(int(daysSinceRelease))

		analysis["latestVersion"] = latest.Version
		analysis["daysSinceRelease"] = daysSinceRelease
		analysis["ageClassification"] = freshness.String()
		analysis["stability"] = vc.GetVersionType(latest.Version).String()
		analysis["isStable"] = vc.IsStable(latest.Version)

		ageDistribution[freshness.String()]++

		analyses[i] = analysis
	}

	result := map[string]any{
		"count":              len(args.Dependencies),
		"successfulAnalysis": countSuccessful(analyses),
		"failedAnalysis":     countFailed(analyses),
		"ageDistribution":    ageDistribution,
		"dependencies":       analyses,
	}

	return successResult(result), nil, nil
}

// handleAnalyzeReleasePatterns handles the analyze_release_patterns tool.
func (s *Server) handleAnalyzeReleasePatterns(ctx context.Context, req *mcp.CallToolRequest, args AnalyzeReleasePatternsArgs) (*mcp.CallToolResult, any, error) {
	observability.Debug("handleAnalyzeReleasePatterns called", "groupId", args.GroupID, "artifactId", args.ArtifactID)

	// Set default limit
	if args.Limit <= 0 {
		args.Limit = 30
	}

	coord := &domain.Coordinates{
		GroupID:    args.GroupID,
		ArtifactID: args.ArtifactID,
		Packaging:  "jar",
	}

	if err := coord.Validate(); err != nil {
		return errorResult("INVALID_COORDINATES", err.Error()), nil, nil
	}

	// Get versions with timestamps
	artifacts, err := s.client.GetVersionsWithTimestamps(ctx, coord, args.Limit)
	if err != nil {
		observability.Error("failed to get versions with timestamps", "error", err)
		if mavencentral.IsNotFound(err) {
			return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
		}
		return errorResult("UPSTREAM_UNAVAILABLE", err.Error()), nil, nil
	}

	if len(artifacts) == 0 {
		return errorResult("NOT_FOUND", fmt.Sprintf("No versions found for %s", coord.String())), nil, nil
	}

	vc := domain.NewVersionComparator()

	// Analyze version types
	stableCount := 0
	rcCount := 0
	betaCount := 0
	alphaCount := 0
	otherCount := 0

	for _, artifact := range artifacts {
		switch vc.GetVersionType(artifact.Version) {
		case domain.StabilityStable:
			stableCount++
		case domain.StabilityRC:
			rcCount++
		case domain.StabilityBeta:
			betaCount++
		case domain.StabilityAlpha:
			alphaCount++
		default:
			otherCount++
		}
	}

	total := float64(len(artifacts))

	// Calculate release intervals
	intervals := make([]int64, 0)
	for i := 0; i < len(artifacts)-1; i++ {
		interval := (artifacts[i].Timestamp - artifacts[i+1].Timestamp) / (24 * 60 * 60 * 1000)
		intervals = append(intervals, interval)
	}

	avgInterval := 0.0
	minInterval := int64(0)
	maxInterval := int64(0)

	if len(intervals) > 0 {
		sum := int64(0)
		minInterval = intervals[0]
		maxInterval = intervals[0]
		for _, interval := range intervals {
			sum += interval
			if interval < minInterval {
				minInterval = interval
			}
			if interval > maxInterval {
				maxInterval = interval
			}
		}
		avgInterval = float64(sum) / float64(len(intervals))
	}

	// Calculate variance for consistency check
	variance := 0.0
	if len(intervals) > 1 {
		for _, interval := range intervals {
			diff := float64(interval) - avgInterval
			variance += diff * diff
		}
		variance /= float64(len(intervals))
	}
	stdDev := math.Sqrt(variance)

	// Determine consistency
	consistency := "regular"
	if stdDev/avgInterval > 0.5 {
		consistency = "variable"
	}
	if stdDev/avgInterval > 1.0 {
		consistency = "erratic"
	}

	// Determine release strategy
	strategy := "standard"
	if float64(stableCount)/total < 0.5 && float64(rcCount)/total > 0.3 {
		strategy = "rc-heavy"
	} else if float64(betaCount)/total > 0.3 {
		strategy = "beta-heavy"
	} else if float64(alphaCount)/total > 0.3 {
		strategy = "alpha-heavy"
	}

	result := map[string]any{
		"dependency":    coord.String(),
		"totalAnalyzed": len(artifacts),
		"versionBreakdown": map[string]any{
			"stable":           stableCount,
			"rc":               rcCount,
			"beta":             betaCount,
			"alpha":            alphaCount,
			"other":            otherCount,
			"stablePercentage": float64(stableCount) / total * 100,
		},
		"releaseCadence": map[string]any{
			"averageIntervalDays": avgInterval,
			"minIntervalDays":     minInterval,
			"maxIntervalDays":     maxInterval,
			"stdDevDays":          stdDev,
			"consistency":         consistency,
		},
		"releaseStrategy": strategy,
		"recommendations": []string{
			fmt.Sprintf("This project uses a %s release strategy", strategy),
			fmt.Sprintf("Releases are %s", consistency),
		},
	}

	return successResult(result), nil, nil
}

// Helper functions

func isBreakingChange(version, previousVersion string) bool {
	if previousVersion == "" {
		return false
	}

	vc := domain.NewVersionComparator()
	return vc.DetermineUpdateType(previousVersion, version) == domain.UpdateMajor
}

func countReleasesInPeriod(artifacts []domain.MavenArtifact, now time.Time, days int) int {
	cutoff := now.AddDate(0, 0, -days).UnixMilli()
	count := 0
	for _, artifact := range artifacts {
		if artifact.Timestamp >= cutoff {
			count++
		}
	}
	return count
}

func generateInsights(artifacts []domain.MavenArtifact, stablePct float64, lastReleaseAge int64, releaseGaps map[string]int) []string {
	insights := []string{}

	if stablePct > 80 {
		insights = append(insights, "Project has excellent stability with mostly stable releases")
	} else if stablePct < 50 {
		insights = append(insights, "Project has many pre-release versions - consider using stable versions for production")
	}

	if lastReleaseAge < 30 {
		insights = append(insights, "Project is actively maintained with recent releases")
	} else if lastReleaseAge > 365 {
		insights = append(insights, "Project appears dormant - no releases in over a year")
	}

	if rapid, ok := releaseGaps["rapid"]; ok && rapid > len(artifacts)/2 {
		insights = append(insights, "Project has rapid release cadence with frequent updates")
	}

	if len(insights) == 0 {
		insights = append(insights, "Standard release pattern detected")
	}

	return insights
}

func countSuccessful(analyses []map[string]any) int {
	count := 0
	for _, a := range analyses {
		if a["status"] == "success" {
			count++
		}
	}
	return count
}

func countFailed(analyses []map[string]any) int {
	count := 0
	for _, a := range analyses {
		if a["status"] != "success" {
			count++
		}
	}
	return count
}
