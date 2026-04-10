package context7

import "fmt"

// MigrationGuidance returns Java-parity orchestration hints for upgrade scenarios.
func MigrationGuidance(dependency, updateType string) map[string]any {
	artifactID := extractArtifactID(dependency)
	query := artifactID + " upgrade guide"
	if updateType == "major" {
		query = artifactID + " migration guide breaking changes"
	}
	fallbackSearch := artifactID + " " + updateType + " version upgrade guide"

	return map[string]any{
		"orchestrationInstructions": fmt.Sprintf(
			"Use resolve-library-id tool with query='%s' and libraryName='%s' to find the library ID. Then use query-docs tool with the returned libraryId and query='%s' to get upgrade instructions. If Context7 doesn't provide sufficient information, perform a web search for '%s'.",
			query,
			artifactID,
			query,
			fallbackSearch,
		),
	}
}

// ModernizationGuidance returns Java-parity orchestration hints for aging/stale dependencies.
func ModernizationGuidance(dependency, ageClassification string) map[string]any {
	artifactID := extractArtifactID(dependency)
	query := artifactID + " modern usage best practices"
	fallbackSearch := artifactID + " latest features best practices"
	if ageClassification == "stale" {
		query = artifactID + " alternatives replacements deprecated"
		fallbackSearch = artifactID + " modernization alternatives"
	}

	return map[string]any{
		"orchestrationInstructions": fmt.Sprintf(
			"Use resolve-library-id tool with query='%s' and libraryName='%s' to find the library ID. Then use query-docs tool with the returned libraryId and query='%s' to get modernization guidance. If Context7 doesn't provide sufficient information, perform a web search for '%s'.",
			query,
			artifactID,
			query,
			fallbackSearch,
		),
	}
}

func extractArtifactID(dependency string) string {
	for i := 0; i < len(dependency); i++ {
		if dependency[i] == ':' {
			remainder := dependency[i+1:]
			for j := 0; j < len(remainder); j++ {
				if remainder[j] == ':' {
					return remainder[:j]
				}
			}
			return remainder
		}
	}
	return dependency
}
