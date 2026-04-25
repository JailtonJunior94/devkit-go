package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var conventionalCommitPattern = regexp.MustCompile(`^([a-z]+)(\(([^)]+)\))?(!)?: (.+)$`)

type ConventionalCommit struct {
	commitType  string
	scope       string
	description string
	breaking    bool
}

type CommitDescriptor struct {
	SHA     string
	Subject string
	Body    string
}

type DefaultVersionResolver struct{}

func ParseConventionalCommit(subject string, body string) (ConventionalCommit, error) {
	matches := conventionalCommitPattern.FindStringSubmatch(subject)
	if matches == nil {
		return ConventionalCommit{}, fmt.Errorf("%w: %s", ErrInvalidConventionalCommit, subject)
	}

	commitType := matches[1]
	if !isSupportedCommitType(commitType) {
		return ConventionalCommit{}, fmt.Errorf("%w: %s", ErrUnsupportedCommitType, commitType)
	}

	return ConventionalCommit{
		commitType:  commitType,
		scope:       matches[3],
		description: matches[5],
		breaking:    matches[4] == "!" || hasBreakingFooter(body),
	}, nil
}

func (c ConventionalCommit) Type() string {
	return c.commitType
}

func (c ConventionalCommit) Scope() string {
	return c.scope
}

func (c ConventionalCommit) Description() string {
	return c.description
}

func (c ConventionalCommit) Breaking() bool {
	return c.breaking
}

func (c ConventionalCommit) Increment() (VersionIncrement, bool) {
	if c.breaking {
		return VersionIncrementMajor, true
	}

	switch c.commitType {
	case "feat":
		return VersionIncrementMinor, true
	case "fix", "perf", "refactor":
		return VersionIncrementPatch, true
	default:
		return "", false
	}
}

func (c CommitDescriptor) ConventionalCommit() (ConventionalCommit, error) {
	return ParseConventionalCommit(c.Subject, c.Body)
}

func (DefaultVersionResolver) ResolveNextVersion(base *SemanticVersion, commits []CommitDescriptor) (SemanticVersion, error) {
	if base == nil {
		return BootstrapVersion(), nil
	}

	highest := rankIncrement("")
	var selected VersionIncrement

	for _, commit := range commits {
		classified, err := commit.ConventionalCommit()
		if err != nil {
			return SemanticVersion{}, err
		}

		increment, ok := classified.Increment()
		if !ok {
			continue
		}

		rank := rankIncrement(increment)
		if rank > highest {
			highest = rank
			selected = increment
		}
	}

	if selected == "" {
		return SemanticVersion{}, ErrNoVersionIncrement
	}

	return base.Bump(selected)
}

func hasBreakingFooter(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "BREAKING CHANGE:") {
			return true
		}
	}

	return false
}

func isSupportedCommitType(commitType string) bool {
	switch commitType {
	case "feat", "fix", "perf", "refactor", "docs", "build", "ci", "chore", "test":
		return true
	default:
		return false
	}
}

func rankIncrement(increment VersionIncrement) int {
	switch increment {
	case VersionIncrementMajor:
		return 3
	case VersionIncrementMinor:
		return 2
	case VersionIncrementPatch:
		return 1
	default:
		return 0
	}
}
