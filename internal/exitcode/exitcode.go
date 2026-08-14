// Package exitcode maps injector failures to stable process exit codes.
package exitcode

import (
	"errors"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/cli"
)

const (
	// Success indicates a successfully managed game session.
	Success = 0
	// Usage indicates invalid command-line input.
	Usage = 2
	// Lifecycle indicates process creation or ownership failure.
	Lifecycle = 3
	// Readiness indicates readiness timeout or early game exit.
	Readiness = 4
	// Injection indicates DLL injection failure.
	Injection = 5
	// Internal indicates another process or Win32 failure.
	Internal = 6
)

// Phase identifies the injector stage that produced an error.
type Phase uint8

const (
	// PhaseLifecycle identifies process creation and ownership errors.
	PhaseLifecycle Phase = iota + 1
	// PhaseReadiness identifies readiness-wait errors.
	PhaseReadiness
	// PhaseInjection identifies DLL injection errors.
	PhaseInjection
	// PhaseInternal identifies other process or Win32 errors.
	PhaseInternal
)

// Error associates a failure with the injector stage that produced it.
type Error struct {
	Phase Phase
	Err   error
}

// Error returns the underlying failure message.
func (e *Error) Error() string { return e.Err.Error() }

// Unwrap returns the underlying failure.
func (e *Error) Unwrap() error { return e.Err }

// Code maps an injector error to its stable process exit code.
func Code(err error) int {
	if err == nil {
		return Success
	}
	if errors.Is(err, cli.ErrUsage) {
		return Usage
	}
	var phase *Error
	if errors.As(err, &phase) {
		switch phase.Phase {
		case PhaseLifecycle:
			return Lifecycle
		case PhaseReadiness:
			return Readiness
		case PhaseInjection:
			return Injection
		case PhaseInternal:
			return Internal
		}
	}
	return Internal
}
