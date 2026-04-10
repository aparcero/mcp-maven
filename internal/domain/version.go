package domain

import (
	"regexp"
	"strconv"
	"strings"
)

// VersionInfo represents version information with type.
type VersionInfo struct {
	Version string
	Type    Stability
}

// MavenArtifact represents an artifact with full metadata.
type MavenArtifact struct {
	Coordinate string
	GroupID    string
	ArtifactID string
	Version    string
	Packaging  string
	Timestamp  int64 // Unix milliseconds
}

// DependencyInfo represents the result of a dependency check.
type DependencyInfo struct {
	Status     string
	GroupID    string
	ArtifactID string
	Version    string
	Exists     bool
	Type       string
	IsStable   bool
	Timestamp  int64
}

// VersionComponents represents parsed version components.
type VersionComponents struct {
	NumericParts []int
	Qualifier    string
}

// VersionComparator handles version comparison and classification.
type VersionComparator struct{}

// NewVersionComparator creates a new version comparator.
func NewVersionComparator() *VersionComparator {
	return &VersionComparator{}
}

// Compare compares two version strings.
// Returns negative if v1 < v2, 0 if equal, positive if v1 > v2.
func (vc *VersionComparator) Compare(v1, v2 string) int {
	if v1 == "" && v2 == "" {
		return 0
	}
	if v1 == "" {
		return -1
	}
	if v2 == "" {
		return 1
	}

	// Simple numeric version comparison
	c1 := vc.ParseVersion(v1)
	c2 := vc.ParseVersion(v2)

	// Compare numeric parts
	maxLen := max(len(c1.NumericParts), len(c2.NumericParts))
	for i := 0; i < maxLen; i++ {
		n1 := 0
		n2 := 0
		if i < len(c1.NumericParts) {
			n1 = c1.NumericParts[i]
		}
		if i < len(c2.NumericParts) {
			n2 = c2.NumericParts[i]
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	// If numeric parts are equal, compare qualifiers
	return vc.compareQualifiers(c1.Qualifier, c2.Qualifier)
}

// compareQualifiers compares version qualifiers.
func (vc *VersionComparator) compareQualifiers(q1, q2 string) int {
	i1 := qualifierInfoFromString(q1)
	i2 := qualifierInfoFromString(q2)

	if i1.rank < i2.rank {
		return -1
	}
	if i1.rank > i2.rank {
		return 1
	}

	if i1.number < i2.number {
		return -1
	}
	if i1.number > i2.number {
		return 1
	}

	if i1.label < i2.label {
		return -1
	}
	if i1.label > i2.label {
		return 1
	}
	return 0
}

// ParseVersion parses a version string into components.
func (vc *VersionComparator) ParseVersion(version string) VersionComponents {
	if version == "" {
		return VersionComponents{NumericParts: []int{}, Qualifier: ""}
	}

	numericParts, qualifier := splitVersionParts(version)
	return VersionComponents{
		NumericParts: numericParts,
		Qualifier:    qualifier,
	}
}

// GetLatest returns the latest version from a slice.
func (vc *VersionComparator) GetLatest(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	latest := versions[0]
	for _, v := range versions[1:] {
		if vc.Compare(v, latest) > 0 {
			latest = v
		}
	}
	return latest
}

// IsStable returns true if the version is considered stable.
func (vc *VersionComparator) IsStable(version string) bool {
	return ClassifyStability(version).IsStable()
}

// GetVersionType returns the version type.
func (vc *VersionComparator) GetVersionType(version string) Stability {
	return ClassifyStability(version)
}

// DetermineUpdateType determines the type of update between two versions.
func (vc *VersionComparator) DetermineUpdateType(current, latest string) UpdateType {
	if current == "" || latest == "" {
		return UpdateUnknown
	}

	if current == latest {
		return UpdateNone
	}

	cmp := vc.Compare(current, latest)
	if cmp >= 0 {
		// Current is newer or same - unusual case
		if cmp == 0 {
			return UpdateNone
		}
		return UpdateUnknown
	}

	c1 := vc.ParseVersion(current)
	c2 := vc.ParseVersion(latest)

	// Find first differing numeric component
	maxLen := max(len(c1.NumericParts), len(c2.NumericParts))
	for i := 0; i < maxLen; i++ {
		n1 := 0
		n2 := 0
		if i < len(c1.NumericParts) {
			n1 = c1.NumericParts[i]
		}
		if i < len(c2.NumericParts) {
			n2 = c2.NumericParts[i]
		}

		if n2 > n1 {
			switch i {
			case 0:
				return UpdateMajor
			case 1:
				return UpdateMinor
			default:
				return UpdatePatch
			}
		}
	}

	// Numeric parts same, check qualifiers
	if c1.Qualifier == "" && c2.Qualifier != "" {
		return UpdateUnknown // Downgrade to pre-release
	}
	if c1.Qualifier != "" && c2.Qualifier == "" {
		return UpdatePatch // Upgrade from pre-release to stable
	}

	// Both have qualifiers, check stability
	return vc.determineQualifierUpdate(c1.Qualifier, c2.Qualifier)
}

// determineQualifierUpdate determines update type based on qualifiers.
func (vc *VersionComparator) determineQualifierUpdate(current, latest string) UpdateType {
	s1 := qualifierInfoFromString(current).rank
	s2 := qualifierInfoFromString(latest).rank

	if s2 > s1 {
		return UpdatePatch // More stable
	}
	if s2 < s1 {
		return UpdateUnknown // Less stable
	}
	return UpdatePatch
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type qualifierInfo struct {
	rank   int
	number int
	label  string
}

func ParseQualifier(version string) string {
	_, qualifier := splitVersionParts(version)
	return qualifier
}

func normalizeQualifier(qualifier string) string {
	return strings.ToLower(strings.TrimSpace(qualifier))
}

func splitVersionParts(version string) ([]int, string) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return []int{}, ""
	}

	tokens := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})

	numericParts := make([]int, 0, len(tokens))
	qualifierTokens := make([]string, 0, 1)
	inQualifier := false

	for _, token := range tokens {
		if token == "" {
			continue
		}

		if inQualifier {
			qualifierTokens = append(qualifierTokens, token)
			continue
		}

		digits := leadingDigits(token)
		switch {
		case digits == token:
			num, _ := strconv.Atoi(digits)
			numericParts = append(numericParts, num)
		case digits != "":
			num, _ := strconv.Atoi(digits)
			numericParts = append(numericParts, num)
			qualifierTokens = append(qualifierTokens, token[len(digits):])
			inQualifier = true
		default:
			qualifierTokens = append(qualifierTokens, token)
			inQualifier = true
		}
	}

	return numericParts, normalizeQualifier(strings.Join(qualifierTokens, "-"))
}

func leadingDigits(s string) string {
	end := 0
	for end < len(s) && isDigit(s[end]) {
		end++
	}
	return s[:end]
}

func qualifierInfoFromString(qualifier string) qualifierInfo {
	normalized := normalizeQualifier(qualifier)
	switch {
	case normalized == "" || isStableQualifier(normalized):
		return qualifierInfo{rank: 5, label: "stable"}
	case qualifierHasPrefix(normalized, "sp"):
		return qualifierInfo{rank: 6, number: trailingNumber(normalized), label: "sp"}
	case qualifierHasPrefix(normalized, "snapshot"):
		return qualifierInfo{rank: 4, number: trailingNumber(normalized), label: "snapshot"}
	case qualifierHasPrefix(normalized, "rc"), qualifierHasPrefix(normalized, "cr"), strings.Contains(normalized, "candidate"):
		return qualifierInfo{rank: 3, number: trailingNumber(normalized), label: "rc"}
	case qualifierHasPrefix(normalized, "milestone"), shortQualifierHasPrefix(normalized, "m"):
		return qualifierInfo{rank: 2, number: trailingNumber(normalized), label: "milestone"}
	case qualifierHasPrefix(normalized, "beta"), shortQualifierHasPrefix(normalized, "b"):
		return qualifierInfo{rank: 1, number: trailingNumber(normalized), label: "beta"}
	case qualifierHasPrefix(normalized, "alpha"), shortQualifierHasPrefix(normalized, "a"), strings.Contains(normalized, "dev"), strings.Contains(normalized, "preview"):
		return qualifierInfo{rank: 0, number: trailingNumber(normalized), label: "alpha"}
	default:
		return qualifierInfo{rank: 0, number: trailingNumber(normalized), label: normalized}
	}
}

func trailingNumber(s string) int {
	re := regexp.MustCompile(`(\d+)$`)
	match := re.FindStringSubmatch(s)
	if len(match) != 2 {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return n
}

// SuccessDependencyInfo creates a successful DependencyInfo.
func SuccessDependencyInfo(coord *Coordinates, version string, exists bool, timestamp int64) DependencyInfo {
	stability := ClassifyStability(version)
	return DependencyInfo{
		Status:     "success",
		GroupID:    coord.GroupID,
		ArtifactID: coord.ArtifactID,
		Version:    version,
		Exists:     exists,
		Type:       stability.String(),
		IsStable:   stability.IsStable(),
		Timestamp:  timestamp,
	}
}

// ErrorDependencyInfo creates an error DependencyInfo.
func ErrorDependencyInfo(coord *Coordinates, version string, err error) DependencyInfo {
	return DependencyInfo{
		Status:     "error",
		GroupID:    coord.GroupID,
		ArtifactID: coord.ArtifactID,
		Version:    version,
		Exists:     false,
		Type:       "",
		IsStable:   false,
		Timestamp:  0,
	}
}
