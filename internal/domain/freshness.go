package domain

// Freshness represents how recently a dependency was updated.
type Freshness string

const (
	FreshnessFresh   Freshness = "fresh"   // < 30 days
	FreshnessCurrent Freshness = "current" // 30-90 days
	FreshnessAging   Freshness = "aging"   // 90-365 days
	FreshnessStale   Freshness = "stale"   // > 365 days
)

// ClassifyFreshness converts days since release to freshness category.
func ClassifyFreshness(daysSinceRelease int) Freshness {
	switch {
	case daysSinceRelease < 30:
		return FreshnessFresh
	case daysSinceRelease < 90:
		return FreshnessCurrent
	case daysSinceRelease < 365:
		return FreshnessAging
	default:
		return FreshnessStale
	}
}

// String returns the string representation of freshness.
func (f Freshness) String() string {
	return string(f)
}

// UpdateType represents the type of version update.
type UpdateType string

const (
	UpdateMajor   UpdateType = "major"
	UpdateMinor   UpdateType = "minor"
	UpdatePatch   UpdateType = "patch"
	UpdateNone    UpdateType = "none"
	UpdateUnknown UpdateType = "unknown"
)

// String returns the string representation of update type.
func (u UpdateType) String() string {
	return string(u)
}
