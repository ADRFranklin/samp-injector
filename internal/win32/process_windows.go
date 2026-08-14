//go:build windows

// Package win32 contains the Windows process and DLL operations used by the injector.
package win32

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createSuspended = 0x00000004
	waitObject      = 0
	waitTimeout     = 258
	stillActive     = 259
	infinite        = 0xffffffff
)

// Process owns a launched GTA process and its kill-on-close Job Object.
type Process struct {
	Handle windows.Handle
	Job    windows.Handle
}

// Launch creates game suspended, assigns it to a kill-on-close Job Object, and resumes it.
func Launch(game, cwd string, args []string) (Process, error) {
	job, err := createKillJob()
	if err != nil {
		return Process{}, fmt.Errorf("create process job: %w", err)
	}

	command := windows.ComposeCommandLine(append([]string{game}, args...))
	commandLine, err := windows.UTF16PtrFromString(command)
	if err != nil {
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("encode command line: %w", err)
	}

	application, err := windows.UTF16PtrFromString(game)
	if err != nil {
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("encode game path: %w", err)
	}

	workingDirectory, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("encode working directory: %w", err)
	}

	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var info windows.ProcessInformation
	if err := windows.CreateProcess(application, commandLine, nil, nil, false, createSuspended, nil, workingDirectory, &startup, &info); err != nil {
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("launch %s: %w", game, err)
	}

	if err := windows.AssignProcessToJobObject(job, info.Process); err != nil {
		windows.TerminateProcess(info.Process, 1)
		windows.CloseHandle(info.Process)
		windows.CloseHandle(info.Thread)
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("assign game to job: %w", err)
	}

	if _, err := windows.ResumeThread(info.Thread); err != nil {
		windows.TerminateProcess(info.Process, 1)
		windows.CloseHandle(info.Thread)
		windows.CloseHandle(info.Process)
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("resume game: %w", err)
	}

	if err := windows.CloseHandle(info.Thread); err != nil {
		windows.TerminateProcess(info.Process, 1)
		windows.CloseHandle(info.Process)
		windows.CloseHandle(job)
		return Process{}, fmt.Errorf("close startup thread: %w", err)
	}

	return Process{Handle: info.Process, Job: job}, nil
}

// Close releases the process and Job Object handles.
func (p Process) Close() error {
	var first error
	if err := windows.CloseHandle(p.Handle); err != nil {
		first = fmt.Errorf("close process: %w", err)
	}
	if err := windows.CloseHandle(p.Job); err != nil && first == nil {
		first = fmt.Errorf("close job: %w", err)
	}
	return first
}

// Wait waits up to timeout for the process to exit.
func (p Process) Wait(timeout time.Duration) (bool, error) {
	result, err := windows.WaitForSingleObject(p.Handle, durationMilliseconds(timeout))
	if err != nil {
		return false, fmt.Errorf("wait for game: %w", err)
	}
	switch result {
	case waitObject:
		return true, nil
	case waitTimeout:
		return false, nil
	default:
		return false, fmt.Errorf("wait for game returned %d", result)
	}
}

// Running reports whether the process has not exited.
func (p Process) Running() (bool, error) {
	finished, err := windows.WaitForSingleObject(p.Handle, 0)
	if err != nil {
		return false, fmt.Errorf("check game status: %w", err)
	}
	return finished != waitObject, nil
}

// ExitCode returns the process exit status after it has exited.
func (p Process) ExitCode() (uint32, error) {
	var code uint32
	if err := windows.GetExitCodeProcess(p.Handle, &code); err != nil {
		return 0, fmt.Errorf("get game exit status: %w", err)
	}
	if code == stillActive {
		return 0, fmt.Errorf("game is still running")
	}
	return code, nil
}

func durationMilliseconds(timeout time.Duration) uint32 {
	if timeout < 0 {
		return infinite
	}
	if timeout == 0 {
		return 0
	}
	ms := timeout / time.Millisecond
	if ms > time.Duration(^uint32(0)) {
		return ^uint32(0) - 1
	}
	return uint32(ms)
}
