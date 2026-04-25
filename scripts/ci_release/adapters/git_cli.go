package adapters

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
)

const (
	recordSeparator = "\x1e"
	fieldSeparator  = "\x1f"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}

	return string(output), nil
}

type GitCLI struct {
	repoPath string
	runner   commandRunner
}

func NewGitCLI(repoPath string) *GitCLI {
	return &GitCLI{
		repoPath: filepath.Clean(repoPath),
		runner:   execCommandRunner{},
	}
}

func (g *GitCLI) LatestTag(ctx context.Context) (*domain.SemanticVersion, error) {
	output, err := g.runGit(ctx, "tag", "--merged", "HEAD", "--list", "v*", "--sort=-version:refname")
	if err != nil {
		return nil, fmt.Errorf("read latest git tag: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" {
			continue
		}

		version, parseErr := domain.ParseSemanticVersion(tag)
		if parseErr != nil {
			return nil, fmt.Errorf("parse latest git tag %q: %w", tag, parseErr)
		}

		return &version, nil
	}

	return nil, nil
}

func (g *GitCLI) CommitsSince(ctx context.Context, base *domain.SemanticVersion) ([]domain.CommitDescriptor, error) {
	args := []string{"log", "--format=%H%x1f%s%x1f%b%x1e"}
	if base == nil {
		args = append(args, "HEAD")
	} else {
		args = append(args, fmt.Sprintf("%s..HEAD", base.String()))
	}

	output, err := g.runGit(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("read git commits: %w", err)
	}

	return parseCommitLog(output), nil
}

func (g *GitCLI) TagExists(ctx context.Context, version domain.SemanticVersion) (bool, error) {
	output, err := g.runGit(ctx, "tag", "--list", version.String())
	if err != nil {
		return false, fmt.Errorf("check git tag %s: %w", version.String(), err)
	}

	return strings.TrimSpace(output) == version.String(), nil
}

func (g *GitCLI) CreateAndPushTag(ctx context.Context, version domain.SemanticVersion) error {
	if _, err := g.runGit(ctx, "tag", version.String()); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create git tag %s: %w", version.String(), domain.ErrTagAlreadyExists)
		}

		return fmt.Errorf("create git tag %s: %w", version.String(), err)
	}

	if _, err := g.runGit(ctx, "push", "origin", fmt.Sprintf("refs/tags/%s", version.String())); err != nil {
		return fmt.Errorf("push git tag %s: %w", version.String(), err)
	}

	return nil
}

func (g *GitCLI) runGit(ctx context.Context, args ...string) (string, error) {
	output, err := g.runner.Run(ctx, "git", append([]string{"-C", g.repoPath}, args...)...)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}

		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return output, nil
}

func parseCommitLog(output string) []domain.CommitDescriptor {
	records := strings.Split(output, recordSeparator)
	commits := make([]domain.CommitDescriptor, 0, len(records))

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.SplitN(record, fieldSeparator, 3)
		if len(fields) != 3 {
			continue
		}

		commits = append(commits, domain.CommitDescriptor{
			SHA:     strings.TrimSpace(fields[0]),
			Subject: strings.TrimSpace(fields[1]),
			Body:    strings.TrimSpace(fields[2]),
		})
	}

	return commits
}
