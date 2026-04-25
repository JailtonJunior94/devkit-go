package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JailtonJunior94/devkit-go/scripts/ci_release/domain"
)

var changelogHeadingPattern = regexp.MustCompile(`^## \[(v[0-9]+\.[0-9]+\.[0-9]+)\](?:\s+-.*)?$`)

type ChangelogReader struct{}

func NewChangelogReader() ChangelogReader {
	return ChangelogReader{}
}

func (ChangelogReader) ReadVersionSection(
	ctx context.Context,
	path string,
	version domain.SemanticVersion,
) (domain.ChangelogSection, error) {
	if err := ctx.Err(); err != nil {
		return domain.ChangelogSection{}, err
	}

	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return domain.ChangelogSection{}, fmt.Errorf("read changelog %s: %w", path, err)
	}

	if err := ctx.Err(); err != nil {
		return domain.ChangelogSection{}, err
	}

	heading := fmt.Sprintf("## [%s]", version.String())
	lines := strings.Split(string(content), "\n")

	occurrences := 0
	foundHeading := ""
	bodyLines := make([]string, 0)
	capturing := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		matches := changelogHeadingPattern.FindStringSubmatch(trimmed)
		if matches != nil {
			if matches[1] == version.String() {
				occurrences++
				if !capturing {
					capturing = true
					foundHeading = heading
					bodyLines = bodyLines[:0]
				}
				continue
			}

			if capturing {
				break
			}

			continue
		}

		if capturing {
			bodyLines = append(bodyLines, trimmed)
		}
	}

	section, buildErr := domain.NewChangelogSection(
		version,
		foundHeading,
		strings.Join(bodyLines, "\n"),
		occurrences,
	)
	if buildErr != nil {
		return domain.ChangelogSection{}, fmt.Errorf("extract changelog section %s from %s: %w", version.String(), path, buildErr)
	}

	return section, nil
}
