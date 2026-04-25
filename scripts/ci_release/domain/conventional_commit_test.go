package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConventionalCommit(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		subject      string
		body         string
		wantType     string
		wantScope    string
		wantBreaking bool
		wantInc      VersionIncrement
		wantHasInc   bool
		wantErr      error
	}{
		{
			name:         "feat with scope",
			subject:      "feat(ci): add release planning",
			wantType:     "feat",
			wantScope:    "ci",
			wantBreaking: false,
			wantInc:      VersionIncrementMinor,
			wantHasInc:   true,
		},
		{
			name:         "fix with bang",
			subject:      "fix!: change release contract",
			wantType:     "fix",
			wantBreaking: true,
			wantInc:      VersionIncrementMajor,
			wantHasInc:   true,
		},
		{
			name:         "breaking footer",
			subject:      "refactor: simplify planner",
			body:         "body\n\nBREAKING CHANGE: new output format",
			wantType:     "refactor",
			wantBreaking: true,
			wantInc:      VersionIncrementMajor,
			wantHasInc:   true,
		},
		{
			name:         "docs ignored for bump",
			subject:      "docs: update release notes",
			wantType:     "docs",
			wantBreaking: false,
			wantHasInc:   false,
		},
		{
			name:    "unsupported type",
			subject: "style: format release flow",
			wantErr: ErrUnsupportedCommitType,
		},
		{
			name:    "invalid format",
			subject: "not conventional",
			wantErr: ErrInvalidConventionalCommit,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseConventionalCommit(testCase.subject, testCase.body)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantType, got.Type())
			assert.Equal(t, testCase.wantScope, got.Scope())
			assert.Equal(t, testCase.wantBreaking, got.Breaking())

			increment, ok := got.Increment()
			assert.Equal(t, testCase.wantHasInc, ok)
			assert.Equal(t, testCase.wantInc, increment)
		})
	}
}

func TestDefaultVersionResolverResolveNextVersion(t *testing.T) {
	t.Parallel()

	base := SemanticVersion{Major: 1, Minor: 2, Patch: 3}

	testCases := []struct {
		name    string
		base    *SemanticVersion
		commits []CommitDescriptor
		want    SemanticVersion
		wantErr error
	}{
		{
			name: "bootstrap ignores commit history",
			base: nil,
			commits: []CommitDescriptor{
				{Subject: "docs: update changelog"},
			},
			want: BootstrapVersion(),
		},
		{
			name: "breaking wins",
			base: &base,
			commits: []CommitDescriptor{
				{Subject: "feat: add planner"},
				{Subject: "fix!: change planner contract"},
			},
			want: SemanticVersion{Major: 2},
		},
		{
			name: "feat without breaking bumps minor",
			base: &base,
			commits: []CommitDescriptor{
				{Subject: "feat: add planner"},
				{Subject: "docs: update docs"},
			},
			want: SemanticVersion{Major: 1, Minor: 3},
		},
		{
			name: "fix perf and refactor bump patch",
			base: &base,
			commits: []CommitDescriptor{
				{Subject: "fix: patch release flow"},
				{Subject: "perf: speed up release flow"},
				{Subject: "refactor: extract parser"},
			},
			want: SemanticVersion{Major: 1, Minor: 2, Patch: 4},
		},
		{
			name: "no eligible bump",
			base: &base,
			commits: []CommitDescriptor{
				{Subject: "docs: update changelog"},
				{Subject: "chore: tidy repo"},
				{Subject: "ci: adjust workflow"},
			},
			wantErr: ErrNoVersionIncrement,
		},
	}

	resolver := DefaultVersionResolver{}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolver.ResolveNextVersion(testCase.base, testCase.commits)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
