package domain

import (
	"fmt"
	"strings"
)

const AllowedReleaseBranch = "main"

type WorkflowContext struct {
	Repository string
	RefName    string
	RunID      string
	Actor      string
}

type ValidationGateStatus struct {
	Lint        bool
	Unit        bool
	Integration bool
	Vulncheck   bool
}

type ChangelogSection struct {
	Version     SemanticVersion
	HeadingLine string
	Body        string
	occurrences int
}

type ReleaseCandidate struct {
	Branch          string
	BaseVersion     *SemanticVersion
	NextVersion     SemanticVersion
	Notes           ChangelogSection
	GateStatus      ValidationGateStatus
	TargetTagExists bool
	Bootstrap       bool
}

func NewChangelogSection(version SemanticVersion, headingLine string, body string, occurrences int) (ChangelogSection, error) {
	switch {
	case occurrences == 0:
		return ChangelogSection{}, fmt.Errorf("%w: %s", ErrMissingChangelogSection, version.String())
	case occurrences > 1:
		return ChangelogSection{}, fmt.Errorf("%w: %s", ErrAmbiguousChangelogSection, version.String())
	}

	expectedHeading := fmt.Sprintf("## [%s]", version.String())
	if headingLine != expectedHeading {
		return ChangelogSection{}, fmt.Errorf("%w: expected heading %s", ErrMissingChangelogSection, expectedHeading)
	}

	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return ChangelogSection{}, fmt.Errorf("%w: %s", ErrMissingChangelogSection, version.String())
	}

	return ChangelogSection{
		Version:     version,
		HeadingLine: headingLine,
		Body:        trimmedBody,
		occurrences: occurrences,
	}, nil
}

func (c ChangelogSection) Markdown() string {
	return c.HeadingLine + "\n\n" + c.Body
}

func (c ChangelogSection) IsUnique() bool {
	return c.occurrences == 1
}

func (v ValidationGateStatus) AllPassed() bool {
	return len(v.FailedGateNames()) == 0
}

func (v ValidationGateStatus) FailedGateNames() []string {
	failed := make([]string, 0, 4)

	if !v.Lint {
		failed = append(failed, "lint")
	}
	if !v.Unit {
		failed = append(failed, "unit")
	}
	if !v.Integration {
		failed = append(failed, "integration")
	}
	if !v.Vulncheck {
		failed = append(failed, "vulncheck")
	}

	return failed
}

func NewReleaseCandidate(
	branch string,
	base *SemanticVersion,
	next SemanticVersion,
	notes ChangelogSection,
	gates ValidationGateStatus,
	targetTagExists bool,
) (ReleaseCandidate, error) {
	if branch != AllowedReleaseBranch {
		return ReleaseCandidate{}, fmt.Errorf("%w: %s", ErrReleaseBranchNotAllowed, branch)
	}

	if !gates.AllPassed() {
		return ReleaseCandidate{}, fmt.Errorf("%w: %s", ErrValidationGateFailed, strings.Join(gates.FailedGateNames(), ","))
	}

	if targetTagExists {
		return ReleaseCandidate{}, fmt.Errorf("%w: %s", ErrTagAlreadyExists, next.String())
	}

	if notes.Version.Compare(next) != 0 || !notes.IsUnique() || strings.TrimSpace(notes.Body) == "" {
		return ReleaseCandidate{}, fmt.Errorf("%w: changelog section does not match target version", ErrInvalidReleaseCandidate)
	}

	bootstrap := base == nil
	if bootstrap {
		if next.Compare(BootstrapVersion()) != 0 {
			return ReleaseCandidate{}, fmt.Errorf("%w: bootstrap release must target %s", ErrInvalidReleaseCandidate, BootstrapVersion().String())
		}
	} else if next.Compare(*base) <= 0 {
		return ReleaseCandidate{}, fmt.Errorf("%w: next version %s must be greater than base %s", ErrInvalidReleaseCandidate, next.String(), base.String())
	}

	return ReleaseCandidate{
		Branch:          branch,
		BaseVersion:     base,
		NextVersion:     next,
		Notes:           notes,
		GateStatus:      gates,
		TargetTagExists: targetTagExists,
		Bootstrap:       bootstrap,
	}, nil
}
