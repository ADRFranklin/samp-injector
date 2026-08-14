//go:build windows

// Package app runs one complete injector session.
package app

import (
	"fmt"
	"io"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/cli"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/exitcode"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/win32"
)

// Run launches, injects, and owns GTA for the duration of the session.
func Run(args []string, stderr io.Writer) int {
	cfg, err := cli.Parse(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(err)
	}
	process, err := win32.Launch(cfg.GamePath, cfg.WorkingDirectory, cfg.GameArgs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseLifecycle, Err: err})
	}
	defer func() { _ = process.Close() }()
	if cfg.WaitForModule {
		if err := win32.WaitForModule(process.Handle, cfg.WaitModule, cfg.WaitTimeout); err != nil {
			fmt.Fprintln(stderr, err)
			return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseReadiness, Err: err})
		}
	}
	if err := win32.Inject(process.Handle, cfg.DLLPath); err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseInjection, Err: err})
	}
	running, err := process.Running()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseInternal, Err: err})
	}
	if !running {
		code, err := process.ExitCode()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseInternal, Err: err})
		}
		return int(code)
	}
	if _, err := process.Wait(-1); err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseInternal, Err: err})
	}
	code, err := process.ExitCode()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.Code(&exitcode.Error{Phase: exitcode.PhaseInternal, Err: err})
	}
	return int(code)
}
