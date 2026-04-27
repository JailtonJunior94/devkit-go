package observability_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityStackFiles(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		contains []string
	}{
		{
			name: "compose provisions isolated lgtm collector and demo",
			path: "docker-compose.observability.yml",
			contains: []string{
				"grafana/otel-lgtm:0.7.5",
				"otel/opentelemetry-collector-contrib:0.121.0",
				"pkg/observability/examples/lgtm-demo/Dockerfile",
				"OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317",
				"devkit-go-observability",
			},
		},
		{
			name: "collector receives otlp and forwards all signals to lgtm",
			path: "collector/otel-collector.yml",
			contains: []string{
				"endpoint: 0.0.0.0:4317",
				"endpoint: 0.0.0.0:4318",
				"endpoint: http://lgtm:4318",
				"traces:",
				"metrics:",
				"logs:",
			},
		},
		{
			name: "prometheus config scrapes collector metrics",
			path: "prometheus/prometheus.yml",
			contains: []string{
				"job_name: 'otel-collector'",
				"targets: [ 'otel-collector:8889' ]",
				"metrics_path: \"/metrics\"",
			},
		},
		{
			name: "grafana datasources point to lgtm services",
			path: "grafana/grafana-datasources.yml",
			contains: []string{
				"type: prometheus",
				"url: http://lgtm:9090",
				"type: tempo",
				"url: http://lgtm:3200",
				"type: loki",
				"url: http://lgtm:3100",
			},
		},
		{
			name: "readme documents start validate and stop operations",
			path: "README.md",
			contains: []string{
				"Ambiente de validacao observavel",
				"docker compose -f deployment/observability/docker-compose.observability.yml up --build -d",
				"curl -H 'x-request-id: demo-req-1'",
				"docker compose -f deployment/observability/docker-compose.observability.yml down",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.path)
			require.NoError(t, err)

			text := string(content)
			for _, want := range tt.contains {
				assert.Contains(t, text, want)
			}
		})
	}
}

func TestComposeDoesNotReferenceRootCompose(t *testing.T) {
	content, err := os.ReadFile("docker-compose.observability.yml")
	require.NoError(t, err)

	assert.NotContains(t, strings.ToLower(string(content)), "docker-compose.yml")
}
