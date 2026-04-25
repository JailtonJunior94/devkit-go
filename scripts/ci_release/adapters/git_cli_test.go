package adapters

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitCLIIntegration(t *testing.T) {
	t.Parallel()

	t.Run("detects bootstrap repository without tags", func(t *testing.T) {
		t.Parallel()

		repoPath, _ := newGitRepository(t)
		gitCLI := NewGitCLI(repoPath)
		firstSHA := gitCommitFile(t, repoPath, "README.md", "bootstrap\n", "docs: bootstrap repository", "")
		secondSHA := gitCommitFile(t, repoPath, "feature.txt", "feature\n", "feat: add release automation", "BREAKING CHANGE: none\n")
		ctx := context.Background()

		latest, err := gitCLI.LatestTag(ctx)
		require.NoError(t, err)
		assert.Nil(t, latest)

		commits, err := gitCLI.CommitsSince(ctx, nil)
		require.NoError(t, err)
		require.Len(t, commits, 2)
		assert.Equal(t, secondSHA, commits[0].SHA)
		assert.Equal(t, firstSHA, commits[1].SHA)
	})

	t.Run("reads commits since existing tag and manages tags", func(t *testing.T) {
		t.Parallel()

		repoPath, remotePath := newGitRepository(t)
		gitCLI := NewGitCLI(repoPath)
		firstSHA := gitCommitFile(t, repoPath, "README.md", "bootstrap\n", "docs: bootstrap repository", "")
		_ = gitCommitFile(t, repoPath, "feature.txt", "feature\n", "feat: add release automation", "BREAKING CHANGE: none\n")
		ctx := context.Background()

		runGit(t, repoPath, "tag", "v0.1.0", firstSHA)

		latest, err := gitCLI.LatestTag(ctx)
		require.NoError(t, err)
		require.NotNil(t, latest)
		assert.Equal(t, "v0.1.0", latest.String())

		commits, err := gitCLI.CommitsSince(ctx, latest)
		require.NoError(t, err)
		require.Len(t, commits, 1)
		assert.Equal(t, "feat: add release automation", commits[0].Subject)

		tagExists, err := gitCLI.TagExists(ctx, mustParseVersion(t, "v0.1.0"))
		require.NoError(t, err)
		assert.True(t, tagExists)

		newVersion := mustParseVersion(t, "v0.2.0")
		require.NoError(t, gitCLI.CreateAndPushTag(ctx, newVersion))

		tagExists, err = gitCLI.TagExists(ctx, newVersion)
		require.NoError(t, err)
		assert.True(t, tagExists)

		remoteTags := runGit(t, remotePath, "tag", "--list")
		assert.Contains(t, remoteTags, "v0.2.0")
	})
}

func newGitRepository(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	remotePath := filepath.Join(root, "remote.git")

	require.NoError(t, os.MkdirAll(repoPath, 0o755))
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.name", "CI Release Tests")
	runGit(t, repoPath, "config", "user.email", "ci-release-tests@example.com")
	runGit(t, repoPath, "config", "commit.gpgsign", "false")
	runGit(t, repoPath, "config", "tag.gpgSign", "false")
	runGit(t, repoPath, "branch", "-M", "main")

	runGit(t, root, "init", "--bare", remotePath)
	runGit(t, repoPath, "remote", "add", "origin", remotePath)

	return repoPath, remotePath
}

func gitCommitFile(t *testing.T, repoPath string, relativePath string, content string, subject string, body string) string {
	t.Helper()

	fullPath := filepath.Join(repoPath, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	runGit(t, repoPath, "add", relativePath)

	args := []string{"commit", "-m", subject}
	if body != "" {
		args = append(args, "-m", strings.TrimSuffix(body, "\n"))
	}
	runGit(t, repoPath, args...)

	return strings.TrimSpace(runGit(t, repoPath, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	return string(output)
}
