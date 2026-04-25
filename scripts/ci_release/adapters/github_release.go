package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
)

type GitHubReleasePublisher struct {
	baseURL    string
	repository string
	token      string
	client     *http.Client
}

func NewGitHubReleasePublisher(baseURL string, repository string, token string) *GitHubReleasePublisher {
	return &GitHubReleasePublisher{
		baseURL:    strings.TrimRight(baseURL, "/"),
		repository: repository,
		token:      token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (g *GitHubReleasePublisher) Publish(
	ctx context.Context,
	version domain.SemanticVersion,
	notes domain.ChangelogSection,
) error {
	payload := map[string]any{
		"tag_name":   version.String(),
		"name":       version.String(),
		"body":       notes.Markdown(),
		"draft":      false,
		"prerelease": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal github release payload: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/repos/%s/releases", g.baseURL, g.repository),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build github release request: %w", err)
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+g.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := g.client.Do(request)
	if err != nil {
		return fmt.Errorf("publish github release %s: %w", version.String(), err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read github release response: %w", err)
	}

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf(
			"publish github release %s: unexpected status %d: %s",
			version.String(),
			response.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	return nil
}
