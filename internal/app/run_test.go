package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/cli"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/event"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/exitcode"
)

type fakeLauncher struct {
	session *fakeSession
	err     error
}

func (f fakeLauncher) Launch(cli.Config) (gameSession, error) { return f.session, f.err }

type fakeSession struct {
	waitErr     error
	injectErr   error
	running     bool
	runningErr  error
	waitGameErr error
	exitCode    uint32
	exitErr     error
	closed      bool
}

type failAfterWriter struct {
	writes int
	limit  int
}

func (w *failAfterWriter) Write(data []byte) (int, error) {
	if w.writes >= w.limit {
		return 0, errors.New("event consumer closed")
	}
	w.writes++
	return len(data), nil
}

func (f *fakeSession) Close() error                                 { f.closed = true; return nil }
func (f *fakeSession) WaitForReadiness(string, time.Duration) error { return f.waitErr }
func (f *fakeSession) Inject(string) error                          { return f.injectErr }
func (f *fakeSession) Running() (bool, error)                       { return f.running, f.runningErr }
func (f *fakeSession) Wait() error                                  { return f.waitGameErr }
func (f *fakeSession) ExitCode() (uint32, error)                    { return f.exitCode, f.exitErr }

func TestRun_version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr, "0.2.0", fakeLauncher{})
	if code != exitcode.Success {
		t.Errorf("run(--version) code = %d, want %d", code, exitcode.Success)
	}
	if got, want := stdout.String(), "samp-injector 0.2.0\n"; got != want {
		t.Errorf("run(--version) stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("run(--version) stderr = %q, want empty", stderr.String())
	}
}

func TestRun_JSONEventsSuccessfulSession(t *testing.T) {
	args := validArgs(t, "--events=json")
	session := &fakeSession{exitCode: 5}
	var stdout, stderr bytes.Buffer

	code := run(args, &stdout, &stderr, "dev", fakeLauncher{session: session})
	if code != 5 {
		t.Errorf("run() code = %d, want GTA exit code 5", code)
	}
	events := decodeEvents(t, stdout.Bytes())
	wantNames := []string{event.Launched, event.Waiting, event.Injecting, event.Injected, event.GameExited}
	if len(events) != len(wantNames) {
		t.Fatalf("run() event count = %d, want %d", len(events), len(wantNames))
	}
	for i, want := range wantNames {
		if events[i].Name != want {
			t.Errorf("run() event %d = %q, want %q", i, events[i].Name, want)
		}
		if events[i].Version != event.ProtocolVersion {
			t.Errorf("run() event %d version = %d, want %d", i, events[i].Version, event.ProtocolVersion)
		}
	}
	if events[1].Module != cli.DefaultWaitModule {
		t.Errorf("waiting module = %q, want %q", events[1].Module, cli.DefaultWaitModule)
	}
	if events[4].Code == nil || *events[4].Code != 5 {
		t.Errorf("game-exited code = %v, want 5", events[4].Code)
	}
	if stderr.Len() != 0 {
		t.Errorf("run() stderr = %q, want empty", stderr.String())
	}
	if !session.closed {
		t.Error("run() did not close the game session")
	}
}

func TestRun_JSONEventsFailures(t *testing.T) {
	for _, test := range []struct {
		name      string
		session   *fakeSession
		wantPhase string
		wantCode  int
	}{
		{name: "readiness", session: &fakeSession{waitErr: errors.New("readiness failed")}, wantPhase: event.PhaseReadiness, wantCode: exitcode.Readiness},
		{name: "injection", session: &fakeSession{injectErr: errors.New("injection failed")}, wantPhase: event.PhaseInjection, wantCode: exitcode.Injection},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(validArgs(t, "--events=json"), &stdout, &stderr, "dev", fakeLauncher{session: test.session})
			if code != test.wantCode {
				t.Errorf("run() code = %d, want %d", code, test.wantCode)
			}
			events := decodeEvents(t, stdout.Bytes())
			last := events[len(events)-1]
			if last.Name != event.Error || last.Phase != test.wantPhase {
				t.Errorf("run() last event = %#v, want error phase %q", last, test.wantPhase)
			}
			if last.Message == "" {
				t.Error("run() error event message is empty")
			}
		})
	}
}

func TestRun_JSONEventsInvalidArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--events=json"}, &stdout, &stderr, "dev", fakeLauncher{})
	if code != exitcode.Usage {
		t.Errorf("run() code = %d, want %d", code, exitcode.Usage)
	}
	events := decodeEvents(t, stdout.Bytes())
	if got, want := len(events), 1; got != want {
		t.Fatalf("run() event count = %d, want %d", got, want)
	}
	if events[0].Name != event.Error || events[0].Phase != event.PhaseArguments {
		t.Errorf("run() event = %#v, want arguments error", events[0])
	}
}

func TestRun_defaultModeDoesNotWriteStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(validArgs(t), &stdout, &stderr, "dev", fakeLauncher{session: &fakeSession{}})
	if code != exitcode.Success {
		t.Errorf("run() code = %d, want %d", code, exitcode.Success)
	}
	if stdout.Len() != 0 {
		t.Errorf("run() stdout = %q, want empty", stdout.String())
	}
}

func TestRun_eventWriterFailureDuringStartupStopsSession(t *testing.T) {
	stdout := &failAfterWriter{}
	session := &fakeSession{}
	var stderr bytes.Buffer

	code := run(validArgs(t, "--events=json"), stdout, &stderr, "dev", fakeLauncher{session: session})
	if code != exitcode.Internal {
		t.Errorf("run() code = %d, want %d", code, exitcode.Internal)
	}
	if !session.closed {
		t.Error("run() did not close session after startup event failure")
	}
	if stderr.Len() == 0 {
		t.Error("run() stderr is empty after startup event failure")
	}
}

func TestRun_eventWriterFailureAfterInjectionDoesNotStopGame(t *testing.T) {
	stdout := &failAfterWriter{limit: 3}
	session := &fakeSession{running: true, exitCode: 9}
	var stderr bytes.Buffer

	code := run(validArgs(t, "--events=json"), stdout, &stderr, "dev", fakeLauncher{session: session})
	if code != 9 {
		t.Errorf("run() code = %d, want GTA exit code 9", code)
	}
	if stderr.Len() == 0 {
		t.Error("run() stderr is empty after runtime event failure")
	}
}

func validArgs(t *testing.T, extra ...string) []string {
	t.Helper()
	dir := t.TempDir()
	game := filepath.Join(dir, "gta_sa.exe")
	dll := filepath.Join(dir, "samp.dll")
	for _, path := range []string{game, dll} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error: %v", path, err)
		}
	}
	args := append([]string(nil), extra...)
	return append(args, "--game", game, "--dll", dll)
}

func decodeEvents(t *testing.T, data []byte) []event.Event {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var events []event.Event
	for {
		var got event.Event
		if err := decoder.Decode(&got); errors.Is(err, io.EOF) {
			return events
		} else if err != nil {
			t.Fatalf("json.Decoder.Decode() error: %v; stdout = %q", err, data)
		}
		events = append(events, got)
	}
}
