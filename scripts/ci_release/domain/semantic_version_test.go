package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSemanticVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		raw     string
		want    SemanticVersion
		wantErr error
	}{
		{
			name: "valid version",
			raw:  "v1.2.3",
			want: SemanticVersion{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:    "missing prefix",
			raw:     "1.2.3",
			wantErr: ErrInvalidSemanticVersion,
		},
		{
			name:    "missing patch",
			raw:     "v1.2",
			wantErr: ErrInvalidSemanticVersion,
		},
		{
			name:    "leading zeros",
			raw:     "v01.2.3",
			wantErr: ErrInvalidSemanticVersion,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseSemanticVersion(testCase.raw)
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

func TestSemanticVersionCompare(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		left   SemanticVersion
		right  SemanticVersion
		expect int
	}{
		{
			name:   "major wins",
			left:   SemanticVersion{Major: 2},
			right:  SemanticVersion{Major: 1, Minor: 9, Patch: 9},
			expect: 1,
		},
		{
			name:   "minor wins",
			left:   SemanticVersion{Major: 1, Minor: 2},
			right:  SemanticVersion{Major: 1, Minor: 3},
			expect: -1,
		},
		{
			name:   "patch wins",
			left:   SemanticVersion{Major: 1, Minor: 2, Patch: 3},
			right:  SemanticVersion{Major: 1, Minor: 2, Patch: 3},
			expect: 0,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expect, testCase.left.Compare(testCase.right))
		})
	}
}

func TestSemanticVersionBump(t *testing.T) {
	t.Parallel()

	base := SemanticVersion{Major: 1, Minor: 2, Patch: 3}

	testCases := []struct {
		name      string
		increment VersionIncrement
		want      SemanticVersion
		wantErr   error
	}{
		{
			name:      "major",
			increment: VersionIncrementMajor,
			want:      SemanticVersion{Major: 2},
		},
		{
			name:      "minor",
			increment: VersionIncrementMinor,
			want:      SemanticVersion{Major: 1, Minor: 3},
		},
		{
			name:      "patch",
			increment: VersionIncrementPatch,
			want:      SemanticVersion{Major: 1, Minor: 2, Patch: 4},
		},
		{
			name:      "invalid",
			increment: "unknown",
			wantErr:   ErrInvalidVersionIncrement,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := base.Bump(testCase.increment)
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
