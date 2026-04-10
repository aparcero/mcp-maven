package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Coordinates represents a Maven artifact coordinate.
type Coordinates struct {
	GroupID    string
	ArtifactID string
	Version    string
	Packaging  string
}

var (
	// Pattern for parsing coordinates in various formats
	coordPattern = regexp.MustCompile(`^([^:]+):([^:]+)(?::([^:]+))?(?::([^:]+))?$`)
)

// ParseCoordinates parses a coordinate string in various formats:
// - "groupId:artifactId:version"
// - "groupId:artifactId"
// - "groupId:artifactId:version:packaging"
func ParseCoordinates(s string) (*Coordinates, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty coordinate string")
	}

	matches := coordPattern.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid coordinate format: %s", s)
	}

	coord := &Coordinates{
		GroupID:    strings.TrimSpace(matches[1]),
		ArtifactID: strings.TrimSpace(matches[2]),
		Version:    strings.TrimSpace(matches[3]),
	}

	if coord.GroupID == "" || coord.ArtifactID == "" {
		return nil, fmt.Errorf("groupId and artifactId are required: %s", s)
	}

	// Packaging is optional
	if matches[4] != "" {
		coord.Packaging = strings.TrimSpace(matches[4])
	} else {
		coord.Packaging = "jar"
	}

	return coord, nil
}

// MustParseCoordinates parses a coordinate string or panics.
func MustParseCoordinates(s string) *Coordinates {
	coord, err := ParseCoordinates(s)
	if err != nil {
		panic(err)
	}
	return coord
}

// String returns the standard coordinate string representation.
func (c *Coordinates) String() string {
	if c.Version != "" {
		return fmt.Sprintf("%s:%s:%s", c.GroupID, c.ArtifactID, c.Version)
	}
	return fmt.Sprintf("%s:%s", c.GroupID, c.ArtifactID)
}

// Path returns the repository path (groupId converted to slashes).
func (c *Coordinates) Path() string {
	groupPath := strings.ReplaceAll(c.GroupID, ".", "/")
	return fmt.Sprintf("%s/%s", groupPath, c.ArtifactID)
}

// PathWithVersion returns the repository path including version.
func (c *Coordinates) PathWithVersion() string {
	if c.Version == "" {
		return c.Path()
	}
	groupPath := strings.ReplaceAll(c.GroupID, ".", "/")
	return fmt.Sprintf("%s/%s/%s", groupPath, c.ArtifactID, c.Version)
}

// Validate checks if the coordinates are valid.
func (c *Coordinates) Validate() error {
	if c.GroupID == "" {
		return fmt.Errorf("groupId is required")
	}
	if c.ArtifactID == "" {
		return fmt.Errorf("artifactId is required")
	}
	return nil
}
