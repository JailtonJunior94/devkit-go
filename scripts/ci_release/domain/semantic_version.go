package domain

import (
	"fmt"
	"regexp"
	"strconv"
)

var semanticVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type VersionIncrement string

const (
	VersionIncrementMajor VersionIncrement = "major"
	VersionIncrementMinor VersionIncrement = "minor"
	VersionIncrementPatch VersionIncrement = "patch"
)

type SemanticVersion struct {
	Major int
	Minor int
	Patch int
}

func ParseSemanticVersion(raw string) (SemanticVersion, error) {
	matches := semanticVersionPattern.FindStringSubmatch(raw)
	if matches == nil {
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidSemanticVersion, raw)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidSemanticVersion, raw)
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidSemanticVersion, raw)
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidSemanticVersion, raw)
	}

	return SemanticVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

func BootstrapVersion() SemanticVersion {
	return SemanticVersion{Major: 0, Minor: 1, Patch: 0}
}

func (v SemanticVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v SemanticVersion) Compare(other SemanticVersion) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}

	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}

	return compareInt(v.Patch, other.Patch)
}

func (v SemanticVersion) Bump(increment VersionIncrement) (SemanticVersion, error) {
	switch increment {
	case VersionIncrementMajor:
		return SemanticVersion{Major: v.Major + 1}, nil
	case VersionIncrementMinor:
		return SemanticVersion{Major: v.Major, Minor: v.Minor + 1}, nil
	case VersionIncrementPatch:
		return SemanticVersion{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}, nil
	default:
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidVersionIncrement, increment)
	}
}

func compareInt(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
