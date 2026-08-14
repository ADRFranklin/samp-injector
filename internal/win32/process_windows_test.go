//go:build windows

package win32

import (
	"testing"
	"time"
)

func TestDurationMilliseconds(t *testing.T) {
	for _, test := range []struct {
		name    string
		timeout time.Duration
		want    uint32
	}{
		{name: "infinite", timeout: -1, want: infinite},
		{name: "poll", timeout: 0, want: 0},
		{name: "positive", timeout: 1500 * time.Millisecond, want: 1500},
		{name: "sub-millisecond", timeout: time.Nanosecond, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := durationMilliseconds(test.timeout); got != test.want {
				t.Errorf("durationMilliseconds(%v) = %d, want %d", test.timeout, got, test.want)
			}
		})
	}
}
