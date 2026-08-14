//go:build windows

package win32

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRetryableModuleSnapshotError(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "bad length", err: windows.ERROR_BAD_LENGTH, want: true},
		{name: "partial copy", err: windows.ERROR_PARTIAL_COPY, want: true},
		{name: "wrapped partial copy", err: fmt.Errorf("snapshot: %w", windows.ERROR_PARTIAL_COPY), want: true},
		{name: "access denied", err: windows.ERROR_ACCESS_DENIED, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := retryableModuleSnapshotError(test.err); got != test.want {
				t.Errorf("retryableModuleSnapshotError(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}
