//go:build windows

// Package win32 contains the Windows process and DLL operations used by the injector.
package win32

import (
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	memCommit        = 0x1000
	memReserve       = 0x2000
	memRelease       = 0x8000
	pageReadWrite    = 0x04
	processAllAccess = 0x001F0FFF
	threadAllAccess  = 0x001F03FF
	infinite         = 0xffffffff
)

var (
	kernel32Inject     = windows.NewLazySystemDLL("kernel32.dll")
	virtualAllocEx     = kernel32Inject.NewProc("VirtualAllocEx")
	virtualFreeEx      = kernel32Inject.NewProc("VirtualFreeEx")
	writeProcessMemory = kernel32Inject.NewProc("WriteProcessMemory")
	createRemoteThread = kernel32Inject.NewProc("CreateRemoteThread")
	getExitCodeThread  = kernel32Inject.NewProc("GetExitCodeThread")
	loadLibraryW       = kernel32Inject.NewProc("LoadLibraryW")
)

// Inject loads path into the process with the normal Windows DLL loader.
func Inject(process windows.Handle, path string) error {
	encoded, err := windows.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("encode DLL path: %w", err)
	}

	bytes := uintptr(len(encoded) * 2)
	remote, _, callErr := virtualAllocEx.Call(uintptr(process), 0, bytes, memCommit|memReserve, pageReadWrite)
	if remote == 0 {
		return fmt.Errorf("allocate remote DLL path: %w", callErr)
	}

	releaseRemote := true
	defer func() {
		if releaseRemote {
			_, _, _ = virtualFreeEx.Call(uintptr(process), remote, 0, memRelease)
		}
	}()

	var written uintptr
	ok, _, callErr := writeProcessMemory.Call(uintptr(process), remote, uintptr(unsafe.Pointer(&encoded[0])), bytes, uintptr(unsafe.Pointer(&written)))
	if ok == 0 || written != bytes {
		return fmt.Errorf("write remote DLL path: %w", callErr)
	}

	localBase, _, moduleName, err := moduleForAddress(windows.GetCurrentProcessId(), loadLibraryW.Addr())
	if err != nil {
		return fmt.Errorf("locate local LoadLibraryW module: %w", err)
	}

	processID, err := windows.GetProcessId(process)
	if err != nil {
		return fmt.Errorf("get target process id: %w", err)
	}

	targetBase, err := moduleAddress(processID, moduleName)
	if err != nil {
		return fmt.Errorf("locate target %s: %w", moduleName, err)
	}

	loaderAddress := targetBase + (loadLibraryW.Addr() - localBase)
	thread, _, callErr := createRemoteThread.Call(uintptr(process), 0, 0, loaderAddress, remote, 0, 0)
	if thread == 0 {
		return fmt.Errorf("create DLL loader thread: %w", callErr)
	}
	defer windows.CloseHandle(windows.Handle(thread))

	result, err := windows.WaitForSingleObject(windows.Handle(thread), durationMilliseconds(30*time.Second))
	if err != nil {
		return fmt.Errorf("wait for DLL loader: %w", err)
	}
	if result == waitTimeout {
		releaseRemote = false
		return fmt.Errorf("DLL loader timed out")
	}
	if result != 0 {
		return fmt.Errorf("DLL loader returned wait status %d", result)
	}

	var module uintptr
	ok, _, callErr = getExitCodeThread.Call(thread, uintptr(unsafe.Pointer(&module)))
	if ok == 0 {
		return fmt.Errorf("read DLL loader result: %w", callErr)
	}
	if module == 0 {
		return fmt.Errorf("LoadLibraryW failed for %s", path)
	}
	loaded, err := HasModule(process, filepath.Base(path))
	if err != nil || !loaded {
		return fmt.Errorf("injected DLL is not present in target: %s", path)
	}

	return nil
}
