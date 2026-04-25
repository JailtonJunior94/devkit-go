package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubReleasePublisherPublish(t *testing.T) {
	t.Parallel()

	version := mustParseVersion(t, "v1.2.3")
	notes, err := domain.NewChangelogSection(version, "## [v1.2.3]", "### Added\n- release body", 1)
	require.NoError(t, err)

	t.Run("publishes release against github api", func(t *testing.T) {
		t.Parallel()

		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "/repos/acme/devkit/releases", request.URL.Path)
			assert.Equal(t, "Bearer secret", request.Header.Get("Authorization"))
			require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
			writer.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		publisher := NewGitHubReleasePublisher(server.URL, "acme/devkit", "secret")
		err := publisher.Publish(context.Background(), version, notes)
		require.NoError(t, err)
		assert.Equal(t, version.String(), payload["tag_name"])
		assert.Equal(t, notes.Markdown(), payload["body"])
	})

	t.Run("returns wrapped error when github api rejects release", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, `{"message":"unprocessable"}`, http.StatusUnprocessableEntity)
		}))
		defer server.Close()

		publisher := NewGitHubReleasePublisher(server.URL, "acme/devkit", "secret")
		err := publisher.Publish(context.Background(), version, notes)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 422")
	})

	t.Run("returns context cancellation from request", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			<-request.Context().Done()
		}))
		defer server.Close()

		publisher := NewGitHubReleasePublisher(server.URL, "acme/devkit", "secret")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := publisher.Publish(ctx, version, notes)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}
