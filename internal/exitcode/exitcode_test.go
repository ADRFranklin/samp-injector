package exitcode

import (
	"errors"
	"testing"
)

func TestCode_mapsWrappedPhases(t *testing.T) {
	for _, test := range []struct {
		phase Phase
		want  int
	}{
		{PhaseLifecycle, Lifecycle}, {PhaseReadiness, Readiness}, {PhaseInjection, Injection}, {PhaseInternal, Internal},
	} {
		if got := Code(&Error{Phase: test.phase, Err: errors.New("failure")}); got != test.want {
			t.Fatalf("Code(%d) = %d, want %d", test.phase, got, test.want)
		}
	}
}
