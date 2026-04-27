package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/adapters"
	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/app"
	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
)

const defaultGitHubAPIURL = "https://api.github.com"

type gateEvaluator struct{}

type cliConfig struct {
	command       string
	repository    string
	refName       string
	runID         string
	actor         string
	changelogPath string
	repoPath      string
	gitHubAPIURL  string
	token         string
	gates         domain.ValidationGateStatus
}

type planJSONOutput struct {
	BaseVersion *string `json:"base_version,omitempty"`
	NextVersion string  `json:"next_version"`
	Bootstrap   bool    `json:"bootstrap"`
	CommitCount int     `json:"commit_count"`
	Notes       string  `json:"notes"`
	Release     bool    `json:"release"`
	SkipReason  string  `json:"skip_reason,omitempty"`
}

type publishJSONOutput struct {
	BaseVersion *string `json:"base_version,omitempty"`
	Version     string  `json:"version"`
	Bootstrap   bool    `json:"bootstrap"`
	Notes       string  `json:"notes"`
	Release     bool    `json:"release"`
	SkipReason  string  `json:"skip_reason,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	getenv func(string) string,
) error {
	config, err := parseCLIConfig(args, getenv)
	if err != nil {
		printUsage(stderr)
		return err
	}

	service, err := buildService(config)
	if err != nil {
		return err
	}

	plan, err := service.PlanRelease(ctx, app.PlanRequest{
		Workflow: domain.WorkflowContext{
			Repository: config.repository,
			RefName:    config.refName,
			RunID:      config.runID,
			Actor:      config.actor,
		},
		GateStatus:    config.gates,
		ChangelogPath: config.changelogPath,
	})
	if err != nil {
		if isReleaseSkippable(err) {
			return writeSkippedRelease(stdout, config.command, err.Error())
		}
		return err
	}

	switch config.command {
	case "plan":
		return writeJSON(stdout, planJSONOutput{
			BaseVersion: versionStringPointer(plan.BaseVersion),
			NextVersion: plan.NextVersion.String(),
			Bootstrap:   plan.Bootstrap,
			CommitCount: len(plan.Commits),
			Notes:       plan.Notes.Markdown(),
			Release:     true,
		})
	case "publish":
		output, publishErr := service.PublishRelease(ctx, plan)
		if publishErr != nil {
			return publishErr
		}

		return writeJSON(stdout, publishJSONOutput{
			BaseVersion: versionStringPointer(plan.BaseVersion),
			Version:     output.Version,
			Bootstrap:   plan.Bootstrap,
			Notes:       output.Notes,
			Release:     true,
		})
	default:
		return fmt.Errorf("unsupported command %q", config.command)
	}
}

func isReleaseSkippable(err error) bool {
	return errors.Is(err, domain.ErrNoVersionIncrement) ||
		errors.Is(err, domain.ErrMissingChangelogSection) ||
		errors.Is(err, domain.ErrTagAlreadyExists)
}

func writeSkippedRelease(stdout io.Writer, command string, reason string) error {
	switch command {
	case "plan":
		return writeJSON(stdout, planJSONOutput{Release: false, SkipReason: reason})
	case "publish":
		return writeJSON(stdout, publishJSONOutput{Release: false, SkipReason: reason})
	default:
		return fmt.Errorf("unsupported command %q", command)
	}
}

func parseCLIConfig(args []string, getenv func(string) string) (cliConfig, error) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		return cliConfig{}, fmt.Errorf("command is required")
	}

	command := args[0]
	if command != "plan" && command != "publish" {
		return cliConfig{}, fmt.Errorf("unsupported command %q", command)
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	config := cliConfig{
		command:       command,
		repository:    strings.TrimSpace(getenv("GITHUB_REPOSITORY")),
		refName:       strings.TrimSpace(getenv("GITHUB_REF_NAME")),
		runID:         strings.TrimSpace(getenv("GITHUB_RUN_ID")),
		actor:         strings.TrimSpace(getenv("GITHUB_ACTOR")),
		changelogPath: "CHANGELOG.md",
		repoPath:      ".",
		gitHubAPIURL:  defaultGitHubAPIURL,
		token:         firstNonEmpty(getenv("GITHUB_TOKEN"), getenv("GH_TOKEN")),
	}

	fs.StringVar(&config.repository, "repository", config.repository, "GitHub repository owner/name")
	fs.StringVar(&config.refName, "ref-name", config.refName, "Git ref name")
	fs.StringVar(&config.runID, "run-id", config.runID, "workflow run identifier")
	fs.StringVar(&config.actor, "actor", config.actor, "workflow actor")
	fs.StringVar(&config.changelogPath, "changelog", config.changelogPath, "path to CHANGELOG.md")
	fs.StringVar(&config.repoPath, "repo-path", config.repoPath, "path to local git repository")
	fs.StringVar(&config.gitHubAPIURL, "github-api-url", config.gitHubAPIURL, "GitHub API base URL")
	fs.StringVar(&config.token, "github-token", config.token, "GitHub token for release publication")
	fs.BoolVar(&config.gates.Lint, "lint", false, "lint gate status")
	fs.BoolVar(&config.gates.Unit, "unit", false, "unit test gate status")
	fs.BoolVar(&config.gates.Integration, "integration", false, "integration test gate status")
	fs.BoolVar(&config.gates.Vulncheck, "vulncheck", false, "vulnerability gate status")

	if err := fs.Parse(args[1:]); err != nil {
		return cliConfig{}, err
	}

	if strings.TrimSpace(config.refName) == "" {
		return cliConfig{}, fmt.Errorf("ref-name is required")
	}
	if strings.TrimSpace(config.changelogPath) == "" {
		return cliConfig{}, fmt.Errorf("changelog path is required")
	}

	config.repoPath = filepath.Clean(config.repoPath)
	config.changelogPath = filepath.Clean(config.changelogPath)

	if command == "publish" {
		if strings.TrimSpace(config.repository) == "" {
			return cliConfig{}, fmt.Errorf("repository is required")
		}
		if strings.TrimSpace(config.token) == "" {
			return cliConfig{}, fmt.Errorf("github token is required")
		}
	}

	return config, nil
}

func buildService(config cliConfig) (*app.Service, error) {
	return app.NewService(
		domain.DefaultVersionResolver{},
		adapters.NewChangelogReader(),
		adapters.NewGitCLI(config.repoPath),
		adapters.NewGitHubReleasePublisher(config.gitHubAPIURL, config.repository, config.token),
		gateEvaluator{},
	)
}

func (gateEvaluator) Validate(_ domain.WorkflowContext, status domain.ValidationGateStatus) error {
	if status.AllPassed() {
		return nil
	}

	return fmt.Errorf(
		"%w: %s",
		domain.ErrValidationGateFailed,
		strings.Join(status.FailedGateNames(), ","),
	)
}

func writeJSON(writer io.Writer, payload any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	return encoder.Encode(payload)
}

func versionStringPointer(version *domain.SemanticVersion) *string {
	if version == nil {
		return nil
	}

	value := version.String()
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func printUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "usage: go run ./scripts/ci_release [--] <plan|publish> [flags]")
}
