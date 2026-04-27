package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIPlanAndPublishIntegration(t *testing.T) {
	t.Parallel()

	t.Run("plan accepts go run separator", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepository(t)
		writeFixtureToRepo(t, repoPath, "bootstrap_changelog.md")
		commitFileCLI(t, repoPath, "README.md", "bootstrap\n", "docs: bootstrap release", "")

		var stdout strings.Builder
		err := run(context.Background(), []string{
			"--",
			"plan",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &stdout, ioDiscard{}, func(string) string { return "" })
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout.String()), &payload))
		assert.Equal(t, "v0.1.0", payload["next_version"])
		assert.Equal(t, true, payload["bootstrap"])
	})

	t.Run("bootstrap plan emits version from real changelog", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepository(t)
		writeFixtureToRepo(t, repoPath, "bootstrap_changelog.md")
		commitFileCLI(t, repoPath, "README.md", "bootstrap\n", "docs: bootstrap release", "")

		var stdout strings.Builder
		err := run(context.Background(), []string{
			"plan",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &stdout, ioDiscard{}, func(string) string { return "" })
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout.String()), &payload))
		assert.Equal(t, "v0.1.0", payload["next_version"])
		assert.Equal(t, true, payload["bootstrap"])
	})

	t.Run("plan skips when no version-bumping commits exist since last tag", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepository(t)
		writeFixtureToRepo(t, repoPath, "existing_tag_changelog.md")
		baselineSHA := commitFileCLI(t, repoPath, "README.md", "baseline\n", "docs: baseline", "")
		runGitCLI(t, repoPath, "tag", "v0.1.0", baselineSHA)
		commitFileCLI(t, repoPath, "docs/notes.md", "notes\n", "docs: extend documentation", "")

		var stdout strings.Builder
		err := run(context.Background(), []string{
			"plan",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &stdout, ioDiscard{}, func(string) string { return "" })
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout.String()), &payload))
		assert.Equal(t, false, payload["release"])
		assert.Contains(t, payload["skip_reason"], "no version increment")
	})

	t.Run("plan fails when next tag already exists", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepository(t)
		writeFixtureToRepo(t, repoPath, "existing_tag_changelog.md")
		baselineSHA := commitFileCLI(t, repoPath, "README.md", "baseline\n", "docs: baseline", "")
		runGitCLI(t, repoPath, "tag", "v0.1.0", baselineSHA)
		commitFileCLI(t, repoPath, "feature.txt", "feature\n", "feat: add release automation", "")
		runGitCLI(t, repoPath, "checkout", "--orphan", "release-existing")
		runGitCLI(t, repoPath, "reset", "--hard")
		commitFileCLI(t, repoPath, "release.txt", "existing release\n", "docs: existing release tag", "")
		runGitCLI(t, repoPath, "tag", "v0.2.0")
		runGitCLI(t, repoPath, "checkout", "main")

		err := run(context.Background(), []string{
			"plan",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &strings.Builder{}, ioDiscard{}, func(string) string { return "" })
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrTagAlreadyExists))
	})

	t.Run("plan fails when changelog section is missing", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepository(t)
		writeFixtureToRepo(t, repoPath, "missing_section_changelog.md")
		baselineSHA := commitFileCLI(t, repoPath, "README.md", "baseline\n", "docs: baseline", "")
		runGitCLI(t, repoPath, "tag", "v0.1.0", baselineSHA)
		commitFileCLI(t, repoPath, "feature.txt", "feature\n", "feat: add release automation", "")

		err := run(context.Background(), []string{
			"plan",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &strings.Builder{}, ioDiscard{}, func(string) string { return "" })
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrMissingChangelogSection))
	})

	t.Run("publish creates tag and github release", func(t *testing.T) {
		t.Parallel()

		repoPath := newCLIRepositoryWithRemote(t)
		writeFixtureToRepo(t, repoPath, "existing_tag_changelog.md")
		baselineSHA := commitFileCLI(t, repoPath, "README.md", "baseline\n", "docs: baseline", "")
		runGitCLI(t, repoPath, "tag", "v0.1.0", baselineSHA)
		runGitCLI(t, repoPath, "push", "origin", "refs/tags/v0.1.0")
		commitFileCLI(t, repoPath, "feature.txt", "feature\n", "feat: add release automation", "")
		runGitCLI(t, repoPath, "push", "origin", "main")

		var requestedBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, "/repos/acme/devkit/releases", request.URL.Path)
			require.NoError(t, json.NewDecoder(request.Body).Decode(&requestedBody))
			writer.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		var stdout strings.Builder
		err := run(context.Background(), []string{
			"publish",
			"--repo-path", repoPath,
			"--changelog", filepath.Join(repoPath, "CHANGELOG.md"),
			"--ref-name", "main",
			"--repository", "acme/devkit",
			"--github-token", "secret",
			"--github-api-url", server.URL,
			"--lint",
			"--unit",
			"--integration",
			"--vulncheck",
		}, &stdout, ioDiscard{}, func(string) string { return "" })
		require.NoError(t, err)
		assert.Equal(t, "v0.2.0", strings.TrimSpace(runGitCLI(t, repoPath, "describe", "--tags", "--abbrev=0")))
		assert.Equal(t, "v0.2.0", requestedBody["tag_name"])
		assert.Contains(t, requestedBody["body"], "## [v0.2.0]")
	})
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func newCLIRepository(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	runGitCLI(t, repoPath, "init")
	runGitCLI(t, repoPath, "config", "user.name", "CI Release Tests")
	runGitCLI(t, repoPath, "config", "user.email", "ci-release-tests@example.com")
	runGitCLI(t, repoPath, "config", "commit.gpgsign", "false")
	runGitCLI(t, repoPath, "config", "tag.gpgSign", "false")
	runGitCLI(t, repoPath, "branch", "-M", "main")

	return repoPath
}

func newCLIRepositoryWithRemote(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	remotePath := filepath.Join(root, "remote.git")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	runGitCLI(t, repoPath, "init")
	runGitCLI(t, repoPath, "config", "user.name", "CI Release Tests")
	runGitCLI(t, repoPath, "config", "user.email", "ci-release-tests@example.com")
	runGitCLI(t, repoPath, "config", "commit.gpgsign", "false")
	runGitCLI(t, repoPath, "config", "tag.gpgSign", "false")
	runGitCLI(t, repoPath, "branch", "-M", "main")
	runGitCLI(t, root, "init", "--bare", remotePath)
	runGitCLI(t, repoPath, "remote", "add", "origin", remotePath)

	return repoPath
}

func writeFixtureToRepo(t *testing.T, repoPath string, fixture string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", "git", fixture))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "CHANGELOG.md"), content, 0o644))
}

func commitFileCLI(t *testing.T, repoPath string, relativePath string, content string, subject string, body string) string {
	t.Helper()

	fullPath := filepath.Join(repoPath, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	runGitCLI(t, repoPath, "add", relativePath)

	args := []string{"commit", "-m", subject}
	if body != "" {
		args = append(args, "-m", strings.TrimSuffix(body, "\n"))
	}
	runGitCLI(t, repoPath, args...)

	return strings.TrimSpace(runGitCLI(t, repoPath, "rev-parse", "HEAD"))
}

func runGitCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	return string(output)
}
