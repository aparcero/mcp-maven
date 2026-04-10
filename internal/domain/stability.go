package domain

import (
	"strings"
)

// Stability represents the stability classification of a version.
type Stability string

const (
	StabilityStable    Stability = "stable"
	StabilityRC        Stability = "rc"
	StabilityBeta      Stability = "beta"
	StabilityAlpha     Stability = "alpha"
	StabilityMilestone Stability = "milestone"
	StabilitySnapshot  Stability = "snapshot"
)

// StabilityFilter represents filtering options for version stability.
type StabilityFilter string

const (
	StabilityAll          StabilityFilter = "ALL"
	StabilityStableOnly   StabilityFilter = "STABLE_ONLY"
	StabilityPreferStable StabilityFilter = "PREFER_STABLE"
)

// ClassifyStability determines the stability level of a version string.
func ClassifyStability(version string) Stability {
	qualifier := normalizeQualifier(ParseQualifier(version))
	if qualifier == "" {
		return StabilityStable
	}

	switch {
	case qualifierHasPrefix(qualifier, "snapshot"):
		return StabilitySnapshot
	case qualifierHasPrefix(qualifier, "rc"), qualifierHasPrefix(qualifier, "cr"), strings.Contains(qualifier, "candidate"):
		return StabilityRC
	case qualifierHasPrefix(qualifier, "beta"), shortQualifierHasPrefix(qualifier, "b"):
		return StabilityBeta
	case qualifierHasPrefix(qualifier, "alpha"), shortQualifierHasPrefix(qualifier, "a"), strings.Contains(qualifier, "dev"), strings.Contains(qualifier, "preview"):
		return StabilityAlpha
	case qualifierHasPrefix(qualifier, "milestone"), shortQualifierHasPrefix(qualifier, "m"):
		return StabilityMilestone
	case isStableQualifier(qualifier), qualifierHasPrefix(qualifier, "sp"):
		return StabilityStable
	default:
		// Unknown qualifiers are treated as pre-release for safety.
		return StabilityAlpha
	}
}

// IsStable returns true if the stability level is considered stable.
func (s Stability) IsStable() bool {
	return s == StabilityStable
}

// IsPreRelease returns true if the stability level is a pre-release.
func (s Stability) IsPreRelease() bool {
	switch s {
	case StabilityRC, StabilityBeta, StabilityAlpha, StabilityMilestone, StabilitySnapshot:
		return true
	default:
		return false
	}
}

// String returns the string representation of the stability.
func (s Stability) String() string {
	return string(s)
}

func isStableQualifier(qualifier string) bool {
	switch qualifier {
	case "", "final", "ga", "release":
		return true
	default:
		return false
	}
}

func qualifierHasPrefix(qualifier, prefix string) bool {
	if qualifier == prefix {
		return true
	}
	if !strings.HasPrefix(qualifier, prefix) || len(qualifier) == len(prefix) {
		return false
	}

	next := qualifier[len(prefix)]
	return isQualifierSeparator(next) || isDigit(next)
}

func shortQualifierHasPrefix(qualifier, prefix string) bool {
	if qualifier == prefix {
		return true
	}
	return len(qualifier) > len(prefix) && strings.HasPrefix(qualifier, prefix) && isDigit(qualifier[len(prefix)])
}

func isQualifierSeparator(b byte) bool {
	return b == '-' || b == '_' || b == '.'
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
