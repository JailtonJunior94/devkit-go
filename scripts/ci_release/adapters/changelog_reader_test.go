package adapters

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangelogReaderReadVersionSection(t *testing.T) {
	t.Parallel()

	reader := NewChangelogReader()
	version := mustParseVersion(t, "v0.1.0")

	testCases := []struct {
		name     string
		fixture  string
		ctx      func() context.Context
		wantBody string
		wantErr  error
	}{
		{
			name:     "extracts section body from real changelog fixture",
			fixture:  "valid.md",
			ctx:      context.Background,
			wantBody: "### Added\n- first release\n\n### Changed\n- release automation",
		},
		{
			name:    "fails when section is duplicated",
			fixture: "duplicate.md",
			ctx:     context.Background,
			wantErr: domain.ErrAmbiguousChangelogSection,
		},
		{
			name:    "fails when section body is empty",
			fixture: "empty.md",
			ctx:     context.Background,
			wantErr: domain.ErrMissingChangelogSection,
		},
		{
			name:    "fails when context is cancelled",
			fixture: "valid.md",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			section, err := reader.ReadVersionSection(
				testCase.ctx(),
				filepath.Join("..", "testdata", "changelog", testCase.fixture),
				version,
			)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "## [v0.1.0]", section.HeadingLine)
			assert.Equal(t, testCase.wantBody, section.Body)
		})
	}
}

func mustParseVersion(t *testing.T, raw string) domain.SemanticVersion {
	t.Helper()

	version, err := domain.ParseSemanticVersion(raw)
	require.NoError(t, err)

	return version
}
