package cmdutil

import (
	"errors"
	"fmt"
)

// ErrSilent aborts the command with a non-zero exit status without printing
// anything further: the command has already explained itself.
var ErrSilent = errors.New("SilentError")

// ErrCancel aborts a command the user backed out of.
var ErrCancel = errors.New("CancelError")

// ErrNoRepo is returned when a command needs a repository and none was given
// or could be inferred from the working directory.
var ErrNoRepo = errors.New("no repository selected; run inside a GitVerse repository or pass --repo OWNER/REPO")

// FlagError signals bad command-line input, so the caller can print usage.
type FlagError struct {
	err error
}

func NewFlagError(err error) *FlagError { return &FlagError{err: err} }

func FlagErrorf(format string, args ...any) *FlagError {
	return NewFlagError(fmt.Errorf(format, args...))
}

func (e *FlagError) Error() string { return e.err.Error() }
func (e *FlagError) Unwrap() error { return e.err }

// AuthError signals a missing or rejected credential.
type AuthError struct {
	err error
}

func NewAuthError(err error) *AuthError { return &AuthError{err: err} }

func (e *AuthError) Error() string { return e.err.Error() }
func (e *AuthError) Unwrap() error { return e.err }

// MutuallyExclusive rejects a combination of flags that cannot be used together.
func MutuallyExclusive(message string, conditions ...bool) error {
	numTrue := 0
	for _, ok := range conditions {
		if ok {
			numTrue++
		}
	}
	if numTrue > 1 {
		return FlagErrorf("%s", message)
	}
	return nil
}
