package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChangelogSection(t *testing.T) {
	t.Parallel()

	version := SemanticVersion{Major: 1, Minor: 2, Patch: 4}

	testCases := []struct {
		name        string
		headingLine string
		body        string
		occurrences int
		wantErr     error
	}{
		{
			name:        "valid section",
			headingLine: "## [v1.2.4]",
			body:        "- release notes",
			occurrences: 1,
		},
		{
			name:        "missing section",
			headingLine: "## [v1.2.4]",
			body:        "- release notes",
			occurrences: 0,
			wantErr:     ErrMissingChangelogSection,
		},
		{
			name:        "ambiguous section",
			headingLine: "## [v1.2.4]",
			body:        "- release notes",
			occurrences: 2,
			wantErr:     ErrAmbiguousChangelogSection,
		},
		{
			name:        "wrong heading",
			headingLine: "## [v1.2.3]",
			body:        "- release notes",
			occurrences: 1,
			wantErr:     ErrMissingChangelogSection,
		},
		{
			name:        "empty body",
			headingLine: "## [v1.2.4]",
			body:        " \n ",
			occurrences: 1,
			wantErr:     ErrMissingChangelogSection,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewChangelogSection(version, testCase.headingLine, testCase.body, testCase.occurrences)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, version, got.Version)
			assert.True(t, got.IsUnique())
		})
	}
}

func TestNewReleaseCandidate(t *testing.T) {
	t.Parallel()

	base := SemanticVersion{Major: 1, Minor: 2, Patch: 3}
	next := SemanticVersion{Major: 1, Minor: 2, Patch: 4}
	notes, err := NewChangelogSection(next, "## [v1.2.4]", "- release notes", 1)
	require.NoError(t, err)

	bootstrapNotes, err := NewChangelogSection(BootstrapVersion(), "## [v0.1.0]", "- first release", 1)
	require.NoError(t, err)

	validGates := ValidationGateStatus{Lint: true, Unit: true, Integration: true, Vulncheck: true}
	failedGates := ValidationGateStatus{Lint: true, Unit: false, Integration: true, Vulncheck: true}

	testCases := []struct {
		name      string
		branch    string
		base      *SemanticVersion
		next      SemanticVersion
		notes     ChangelogSection
		gates     ValidationGateStatus
		tagExists bool
		wantErr   error
	}{
		{
			name:   "valid release candidate",
			branch: AllowedReleaseBranch,
			base:   &base,
			next:   next,
			notes:  notes,
			gates:  validGates,
		},
		{
			name:    "branch not allowed",
			branch:  "develop",
			base:    &base,
			next:    next,
			notes:   notes,
			gates:   validGates,
			wantErr: ErrReleaseBranchNotAllowed,
		},
		{
			name:    "gate failed",
			branch:  AllowedReleaseBranch,
			base:    &base,
			next:    next,
			notes:   notes,
			gates:   failedGates,
			wantErr: ErrValidationGateFailed,
		},
		{
			name:      "tag already exists",
			branch:    AllowedReleaseBranch,
			base:      &base,
			next:      next,
			notes:     notes,
			gates:     validGates,
			tagExists: true,
			wantErr:   ErrTagAlreadyExists,
		},
		{
			name:    "bootstrap requires v0.1.0",
			branch:  AllowedReleaseBranch,
			base:    nil,
			next:    next,
			notes:   notes,
			gates:   validGates,
			wantErr: ErrInvalidReleaseCandidate,
		},
		{
			name:   "valid bootstrap release candidate",
			branch: AllowedReleaseBranch,
			base:   nil,
			next:   BootstrapVersion(),
			notes:  bootstrapNotes,
			gates:  validGates,
		},
		{
			name:    "next version must be greater than base",
			branch:  AllowedReleaseBranch,
			base:    &base,
			next:    base,
			notes:   notes,
			gates:   validGates,
			wantErr: ErrInvalidReleaseCandidate,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewReleaseCandidate(
				testCase.branch,
				testCase.base,
				testCase.next,
				testCase.notes,
				testCase.gates,
				testCase.tagExists,
			)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.branch, got.Branch)
			assert.Equal(t, testCase.next, got.NextVersion)
		})
	}
}
