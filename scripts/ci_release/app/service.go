package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
)

type VersionResolver interface {
	ResolveNextVersion(base *domain.SemanticVersion, commits []domain.CommitDescriptor) (domain.SemanticVersion, error)
}

type ChangelogReader interface {
	ReadVersionSection(ctx context.Context, path string, version domain.SemanticVersion) (domain.ChangelogSection, error)
}

type GitMetadataReader interface {
	LatestTag(ctx context.Context) (*domain.SemanticVersion, error)
	CommitsSince(ctx context.Context, base *domain.SemanticVersion) ([]domain.CommitDescriptor, error)
	TagExists(ctx context.Context, version domain.SemanticVersion) (bool, error)
	CreateAndPushTag(ctx context.Context, version domain.SemanticVersion) error
}

type ReleasePublisher interface {
	Publish(ctx context.Context, version domain.SemanticVersion, notes domain.ChangelogSection) error
}

type GateEvaluator interface {
	Validate(ctx domain.WorkflowContext, status domain.ValidationGateStatus) error
}

type Service struct {
	versionResolver  VersionResolver
	changelogReader  ChangelogReader
	gitMetadata      GitMetadataReader
	releasePublisher ReleasePublisher
	gateEvaluator    GateEvaluator
}

type PlanRequest struct {
	Workflow      domain.WorkflowContext
	GateStatus    domain.ValidationGateStatus
	ChangelogPath string
}

type ReleasePlan struct {
	BaseVersion *domain.SemanticVersion
	NextVersion domain.SemanticVersion
	Notes       domain.ChangelogSection
	Commits     []domain.CommitDescriptor
	Bootstrap   bool
}

type ReleaseOutput struct {
	Version string
	Notes   string
}

func NewService(
	versionResolver VersionResolver,
	changelogReader ChangelogReader,
	gitMetadata GitMetadataReader,
	releasePublisher ReleasePublisher,
	gateEvaluator GateEvaluator,
) (*Service, error) {
	if err := requireDependency("versionResolver", versionResolver); err != nil {
		return nil, err
	}
	if err := requireDependency("changelogReader", changelogReader); err != nil {
		return nil, err
	}
	if err := requireDependency("gitMetadata", gitMetadata); err != nil {
		return nil, err
	}
	if err := requireDependency("releasePublisher", releasePublisher); err != nil {
		return nil, err
	}
	if err := requireDependency("gateEvaluator", gateEvaluator); err != nil {
		return nil, err
	}

	return &Service{
		versionResolver:  versionResolver,
		changelogReader:  changelogReader,
		gitMetadata:      gitMetadata,
		releasePublisher: releasePublisher,
		gateEvaluator:    gateEvaluator,
	}, nil
}

func (s *Service) PlanRelease(ctx context.Context, request PlanRequest) (ReleasePlan, error) {
	if strings.TrimSpace(request.ChangelogPath) == "" {
		return ReleasePlan{}, fmt.Errorf("changelog path is required")
	}

	if err := s.gateEvaluator.Validate(request.Workflow, request.GateStatus); err != nil {
		return ReleasePlan{}, err
	}

	baseVersion, err := s.gitMetadata.LatestTag(ctx)
	if err != nil {
		return ReleasePlan{}, err
	}

	commits, err := s.gitMetadata.CommitsSince(ctx, baseVersion)
	if err != nil {
		return ReleasePlan{}, err
	}

	nextVersion, err := s.versionResolver.ResolveNextVersion(baseVersion, commits)
	if err != nil {
		return ReleasePlan{}, err
	}

	notes, err := s.changelogReader.ReadVersionSection(ctx, request.ChangelogPath, nextVersion)
	if err != nil {
		return ReleasePlan{}, err
	}

	tagExists, err := s.gitMetadata.TagExists(ctx, nextVersion)
	if err != nil {
		return ReleasePlan{}, err
	}

	_, err = domain.NewReleaseCandidate(
		request.Workflow.RefName,
		baseVersion,
		nextVersion,
		notes,
		request.GateStatus,
		tagExists,
	)
	if err != nil {
		return ReleasePlan{}, err
	}

	return ReleasePlan{
		BaseVersion: baseVersion,
		NextVersion: nextVersion,
		Notes:       notes,
		Commits:     commits,
		Bootstrap:   baseVersion == nil,
	}, nil
}

func (s *Service) PublishRelease(ctx context.Context, plan ReleasePlan) (ReleaseOutput, error) {
	if err := s.gitMetadata.CreateAndPushTag(ctx, plan.NextVersion); err != nil {
		return ReleaseOutput{}, err
	}

	if err := s.releasePublisher.Publish(ctx, plan.NextVersion, plan.Notes); err != nil {
		return ReleaseOutput{}, err
	}

	return ReleaseOutput{
		Version: plan.NextVersion.String(),
		Notes:   plan.Notes.Markdown(),
	}, nil
}

func requireDependency(name string, dependency any) error {
	if dependency == nil {
		return fmt.Errorf("%s is required", name)
	}

	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return fmt.Errorf("%s is required", name)
		}
	}

	return nil
}
