package cmd

import "errors"

// Exit codes returned by mkdb. These are part of the CLI's machine interface
// and are documented in the README for use by scripts and AI agents.
const (
	exitOK                = 0 // success
	exitGeneral           = 1 // unspecified error
	exitNotFound          = 2 // the requested container/resource does not exist
	exitDockerUnavailable = 3 // the Docker daemon could not be reached
	exitTimeout           = 4 // an operation timed out (e.g. readiness wait)
)

// exitError wraps an error with a specific process exit code. Commands return
// it (via withExitCode) when they want a non-default exit status; Execute maps
// it to os.Exit.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// withExitCode tags err with a process exit code. It returns nil when err is
// nil so it is safe to wrap unconditionally.
func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

// exitCodeFor returns the process exit code for a command error, defaulting to
// exitGeneral for untagged errors.
func exitCodeFor(err error) int {
	if err == nil {
		return exitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return exitGeneral
}
