package domain

import "errors"

var (
	ErrInvalidSemanticVersion    = errors.New("invalid semantic version")
	ErrInvalidVersionIncrement   = errors.New("invalid version increment")
	ErrInvalidConventionalCommit = errors.New("invalid conventional commit")
	ErrNoVersionIncrement        = errors.New("no version increment")
	ErrUnsupportedCommitType     = errors.New("unsupported commit type")
	ErrMissingChangelogSection   = errors.New("missing changelog section")
	ErrAmbiguousChangelogSection = errors.New("ambiguous changelog section")
	ErrReleaseBranchNotAllowed   = errors.New("release branch not allowed")
	ErrValidationGateFailed      = errors.New("validation gate failed")
	ErrTagAlreadyExists          = errors.New("tag already exists")
	ErrInvalidReleaseCandidate   = errors.New("invalid release candidate")
)
