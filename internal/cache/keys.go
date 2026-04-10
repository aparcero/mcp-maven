package cache

import (
	"fmt"
	"strings"
)

// Key generates cache keys for different data types.
type Key struct{}

// AllVersions generates a cache key for all versions of an artifact.
func (Key) AllVersions(groupID, artifactID, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:all:%s:%s:%s", groupID, artifactID, packaging)
}

// VersionCheck generates a cache key for a single version existence check.
func (Key) VersionCheck(groupID, artifactID, version, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:check:%s:%s:%s:%s", groupID, artifactID, version, packaging)
}

// HistoricalData generates a cache key for historical version data.
func (Key) HistoricalData(groupID, artifactID string, limit int, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:history:%s:%s:%d:%s", groupID, artifactID, limit, packaging)
}

// Timestamp generates a cache key for a POM timestamp.
func (Key) Timestamp(groupID, artifactID, version string) string {
	return fmt.Sprintf("timestamp:%s:%s:%s", groupID, artifactID, version)
}

// NormalizeGroupID converts a group ID to a path-like string for caching.
func (Key) NormalizeGroupID(groupID string) string {
	return strings.ReplaceAll(groupID, ".", "/")
}
