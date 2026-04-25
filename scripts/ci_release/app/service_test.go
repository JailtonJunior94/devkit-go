package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServiceRequiresDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewService(nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionResolver")
}

func TestPlanRelease(t *testing.T) {
	t.Parallel()

	base := domain.SemanticVersion{Major: 1, Minor: 2, Patch: 3}
	next := domain.SemanticVersion{Major: 1, Minor: 3}
	notes, err := domain.NewChangelogSection(next, "## [v1.3.0]", "- release notes", 1)
	require.NoError(t, err)

	gates := domain.ValidationGateStatus{Lint: true, Unit: true, Integration: true, Vulncheck: true}
	workflow := domain.WorkflowContext{RefName: domain.AllowedReleaseBranch}

	testCases := []struct {
		name          string
		latestTag     *domain.SemanticVersion
		commits       []domain.CommitDescriptor
		notes         domain.ChangelogSection
		tagExists     bool
		gateErr       error
		wantErr       error
		wantVersion   domain.SemanticVersion
		wantBootstrap bool
	}{
		{
			name:        "planned release from existing tag",
			latestTag:   &base,
			commits:     []domain.CommitDescriptor{{Subject: "feat: add release plan"}},
			notes:       notes,
			wantVersion: next,
		},
		{
			name:          "bootstrap release",
			latestTag:     nil,
			commits:       []domain.CommitDescriptor{{Subject: "docs: update changelog"}},
			notes:         mustChangelogSection(t, domain.BootstrapVersion(), "## [v0.1.0]", "- bootstrap release", 1),
			wantVersion:   domain.BootstrapVersion(),
			wantBootstrap: true,
		},
		{
			name:      "tag already exists",
			latestTag: &base,
			commits:   []domain.CommitDescriptor{{Subject: "feat: add release plan"}},
			notes:     notes,
			tagExists: true,
			wantErr:   domain.ErrTagAlreadyExists,
		},
		{
			name:      "gate evaluator fails",
			latestTag: &base,
			gateErr:   domain.ErrValidationGateFailed,
			wantErr:   domain.ErrValidationGateFailed,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gitReader := &stubGitMetadataReader{
				latestTag: testCase.latestTag,
				commits:   testCase.commits,
				tagExists: testCase.tagExists,
			}
			changelogReader := &stubChangelogReader{section: testCase.notes}

			service, err := NewService(
				domain.DefaultVersionResolver{},
				changelogReader,
				gitReader,
				&stubReleasePublisher{},
				stubGateEvaluator{err: testCase.gateErr},
			)
			require.NoError(t, err)

			plan, err := service.PlanRelease(context.Background(), PlanRequest{
				Workflow:      workflow,
				GateStatus:    gates,
				ChangelogPath: "CHANGELOG.md",
			})
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, testCase.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantVersion, plan.NextVersion)
			assert.Equal(t, testCase.wantBootstrap, plan.Bootstrap)
			assert.Equal(t, "CHANGELOG.md", changelogReader.path)
		})
	}
}

func TestPublishRelease(t *testing.T) {
	t.Parallel()

	next := domain.SemanticVersion{Major: 1, Minor: 3}
	notes := mustChangelogSection(t, next, "## [v1.3.0]", "- release notes", 1)

	gitReader := &stubGitMetadataReader{}
	publisher := &stubReleasePublisher{}

	service, err := NewService(
		domain.DefaultVersionResolver{},
		&stubChangelogReader{section: notes},
		gitReader,
		publisher,
		stubGateEvaluator{},
	)
	require.NoError(t, err)

	output, err := service.PublishRelease(context.Background(), ReleasePlan{
		NextVersion: next,
		Notes:       notes,
	})
	require.NoError(t, err)
	assert.Equal(t, next.String(), output.Version)
	assert.Equal(t, notes.Markdown(), output.Notes)
	assert.Equal(t, next, gitReader.createdTag)
	assert.Equal(t, next, publisher.version)
}

type stubGitMetadataReader struct {
	latestTag  *domain.SemanticVersion
	commits    []domain.CommitDescriptor
	tagExists  bool
	createdTag domain.SemanticVersion
}

func (s *stubGitMetadataReader) LatestTag(context.Context) (*domain.SemanticVersion, error) {
	return s.latestTag, nil
}

func (s *stubGitMetadataReader) CommitsSince(context.Context, *domain.SemanticVersion) ([]domain.CommitDescriptor, error) {
	return s.commits, nil
}

func (s *stubGitMetadataReader) TagExists(context.Context, domain.SemanticVersion) (bool, error) {
	return s.tagExists, nil
}

func (s *stubGitMetadataReader) CreateAndPushTag(_ context.Context, version domain.SemanticVersion) error {
	s.createdTag = version
	return nil
}

type stubChangelogReader struct {
	section domain.ChangelogSection
	path    string
}

func (s *stubChangelogReader) ReadVersionSection(_ context.Context, path string, version domain.SemanticVersion) (domain.ChangelogSection, error) {
	s.path = path
	return s.section, nil
}

type stubReleasePublisher struct {
	version domain.SemanticVersion
	notes   domain.ChangelogSection
}

func (s *stubReleasePublisher) Publish(_ context.Context, version domain.SemanticVersion, notes domain.ChangelogSection) error {
	s.version = version
	s.notes = notes
	return nil
}

type stubGateEvaluator struct {
	err error
}

func (s stubGateEvaluator) Validate(domain.WorkflowContext, domain.ValidationGateStatus) error {
	return s.err
}

func mustChangelogSection(
	t *testing.T,
	version domain.SemanticVersion,
	headingLine string,
	body string,
	occurrences int,
) domain.ChangelogSection {
	t.Helper()

	section, err := domain.NewChangelogSection(version, headingLine, body, occurrences)
	require.NoError(t, err)

	return section
}
