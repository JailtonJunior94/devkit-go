package worker

import "errors"

var (
	ErrDuplicateName  = errors.New("worker: duplicate name")
	ErrStopTimeout    = errors.New("worker: shutdown timeout exceeded")
	ErrAlreadyStarted = errors.New("worker: already started")
)
