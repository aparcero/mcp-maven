package cache

import "fmt"

// AllVersionsKey generates a cache key for all versions of an artifact.
func AllVersionsKey(groupID, artifactID, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:all:%s:%s:%s", groupID, artifactID, packaging)
}

// VersionCheckKey generates a cache key for a single version existence check.
func VersionCheckKey(groupID, artifactID, version, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:check:%s:%s:%s:%s", groupID, artifactID, version, packaging)
}

// HistoricalDataKey generates a cache key for historical version data.
func HistoricalDataKey(groupID, artifactID string, limit int, packaging string) string {
	if packaging == "" {
		packaging = "jar"
	}
	return fmt.Sprintf("versions:history:%s:%s:%d:%s", groupID, artifactID, limit, packaging)
}

// TimestampKey generates a cache key for a POM timestamp.
func TimestampKey(groupID, artifactID, version string) string {
	return fmt.Sprintf("timestamp:%s:%s:%s", groupID, artifactID, version)
}
