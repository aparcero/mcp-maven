package domain

import (
	"time"
)

// ReleaseGap classifies gaps between releases.
type ReleaseGap string

const (
	ReleaseGapRapid    ReleaseGap = "rapid"     // < 30% of average
	ReleaseGapNormal   ReleaseGap = "normal"    // 30-150% of average
	ReleaseGapSlow     ReleaseGap = "slow"      // 150-300% of average
	ReleaseGapMajorGap ReleaseGap = "major_gap" // > 300% of average
)

// ClassifyReleaseGap classifies a release gap based on days since previous and average interval.
func ClassifyReleaseGap(daysSincePrevious int, averageInterval float64) ReleaseGap {
	if averageInterval == 0 || float64(daysSincePrevious) <= 0 {
		return ReleaseGapNormal
	}

	ratio := float64(daysSincePrevious) / averageInterval
	switch {
	case ratio <= 0.3:
		return ReleaseGapRapid
	case ratio <= 1.5:
		return ReleaseGapNormal
	case ratio <= 3.0:
		return ReleaseGapSlow
	default:
		return ReleaseGapMajorGap
	}
}

// TrendDirection represents velocity trend.
type TrendDirection string

const (
	TrendAccelerating TrendDirection = "accelerating"
	TrendStable       TrendDirection = "stable"
	TrendDeclining    TrendDirection = "declining"
	TrendErratic      TrendDirection = "erratic"
)

// ActivityLevel represents release activity level.
type ActivityLevel string

const (
	ActivityVeryActive ActivityLevel = "very_active"
	ActivityActive     ActivityLevel = "active"
	ActivityModerate   ActivityLevel = "moderate"
	ActivityLow        ActivityLevel = "low"
	ActivityDormant    ActivityLevel = "dormant"
)

// String returns the string representation of ActivityLevel.
func (a ActivityLevel) String() string {
	return string(a)
}

// ClassifyActivityLevel classifies activity based on recent releases.
func ClassifyActivityLevel(releasesLastMonth, releasesLastQuarter int) ActivityLevel {
	switch {
	case releasesLastMonth >= 3:
		return ActivityVeryActive
	case releasesLastMonth >= 1:
		return ActivityActive
	case releasesLastQuarter >= 2:
		return ActivityModerate
	case releasesLastQuarter >= 1:
		return ActivityLow
	default:
		return ActivityDormant
	}
}

// TimelineEntry represents a single version in a timeline.
type TimelineEntry struct {
	Version           string
	VersionType       Stability
	ReleaseDate       time.Time
	RelativeTime      string
	DaysSincePrevious int64
	IsBreakingChange  bool
	ReleaseGap        ReleaseGap
}

// VelocityTrend analyzes release velocity changes.
type VelocityTrend struct {
	Trend              TrendDirection
	Description        string
	RecentVelocity     float64
	HistoricalVelocity float64
	ChangePercentage   float64
}

// StabilityPattern analyzes version stability patterns.
type StabilityPattern struct {
	StablePercentage     float64
	PrereleasePattern    string
	StableReleasePattern string
	Recommendation       string
}

// RecentActivity summarizes recent release activity.
type RecentActivity struct {
	ReleasesLastMonth   int
	ReleasesLastQuarter int
	ActivityLevel       ActivityLevel
	LastReleaseAge      int64
	ActivityDescription string
}

// VersionTimelineAnalysis provides temporal analysis.
type VersionTimelineAnalysis struct {
	Dependency           string
	TotalVersions        int
	VersionsReturned     int
	TimeSpanMonths       int
	VersionTimeline      []TimelineEntry
	ReleaseVelocityTrend VelocityTrend
	StabilityPattern     StabilityPattern
	RecentActivity       RecentActivity
	Insights             []string
}

// FormatRelativeTime formats a relative time description.
func FormatRelativeTime(releaseDate time.Time) string {
	now := time.Now()
	days := int(now.Sub(releaseDate).Hours() / 24)

	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days <= 7:
		return "days ago"
	case days <= 30:
		return "weeks ago"
	case days <= 365:
		return "months ago"
	default:
		return "years ago"
	}
}
