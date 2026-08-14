//go:build windows

// Package win32 contains the Windows process and DLL operations used by the injector.
package win32

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	th32csSnapModule   = 0x00000008
	th32csSnapModule32 = 0x00000010
)

// HasModule reports whether an exact module basename is loaded in process.
func HasModule(process windows.Handle, name string) (bool, error) {
	pid, err := windows.GetProcessId(process)
	if err != nil {
		return false, fmt.Errorf("get game process id: %w", err)
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapModule|th32csSnapModule32, pid)
	if err != nil {
		return false, fmt.Errorf("create module snapshot: %w", err)
	}

	defer windows.CloseHandle(snapshot)
	entry := windows.ModuleEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Module32First(snapshot, &entry); err != nil {
		return false, fmt.Errorf("read first module: %w", err)
	}

	for {
		if strings.EqualFold(windows.UTF16ToString(entry.Module[:]), name) {
			return true, nil
		}
		if err := windows.Module32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return false, nil
			}
			return false, fmt.Errorf("read module: %w", err)
		}
	}
}

func moduleAddress(processID uint32, name string) (uintptr, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapModule|th32csSnapModule32, processID)
	if err != nil {
		return 0, fmt.Errorf("create module snapshot: %w", err)
	}

	defer windows.CloseHandle(snapshot)
	entry := windows.ModuleEntry32{Size: uint32(unsafe.Sizeof(windows.ModuleEntry32{}))}
	for err := windows.Module32First(snapshot, &entry); err == nil; err = windows.Module32Next(snapshot, &entry) {
		if strings.EqualFold(windows.UTF16ToString(entry.Module[:]), name) {
			return entry.ModBaseAddr, nil
		}
	}

	return 0, fmt.Errorf("module %s is not loaded", name)
}

func moduleForAddress(processID uint32, address uintptr) (uintptr, uintptr, string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapModule|th32csSnapModule32, processID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("create module snapshot: %w", err)
	}

	defer windows.CloseHandle(snapshot)
	entry := windows.ModuleEntry32{Size: uint32(unsafe.Sizeof(windows.ModuleEntry32{}))}
	for err := windows.Module32First(snapshot, &entry); err == nil; err = windows.Module32Next(snapshot, &entry) {
		base := entry.ModBaseAddr
		if address >= base && address-base < uintptr(entry.ModBaseSize) {
			return base, uintptr(entry.ModBaseSize), windows.UTF16ToString(entry.Module[:]), nil
		}
	}

	return 0, 0, "", fmt.Errorf("module containing address 0x%x is not loaded", address)
}

// WaitForModule waits for an exact module basename until timeout expires.
func WaitForModule(process windows.Handle, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		found, err := HasModule(process, name)
		if err == nil && found {
			return nil
		}
		if err != nil && !retryableModuleSnapshotError(err) {
			return fmt.Errorf("inspect modules while waiting for %s: %w", name, err)
		}

		finished, waitErr := windows.WaitForSingleObject(process, 0)
		if waitErr != nil {
			return fmt.Errorf("check game while waiting for %s: %w", name, waitErr)
		}
		if finished == waitObject {
			return fmt.Errorf("game exited before %s loaded", name)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", name)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func retryableModuleSnapshotError(err error) bool {
	return errors.Is(err, windows.ERROR_BAD_LENGTH) || errors.Is(err, windows.ERROR_PARTIAL_COPY)
}
