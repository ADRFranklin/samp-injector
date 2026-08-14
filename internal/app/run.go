// Package app runs one complete injector session.
package app

import (
	"fmt"
	"io"
	"time"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/cli"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/event"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/exitcode"
)

type launcher interface {
	Launch(cli.Config) (gameSession, error)
}

type gameSession interface {
	Close() error
	WaitForReadiness(string, time.Duration) error
	Inject(string) error
	Running() (bool, error)
	Wait() error
	ExitCode() (uint32, error)
}

type eventEmitter struct {
	sink   event.Sink
	stderr io.Writer
}

func (e *eventEmitter) startup(record event.Event) error {
	if err := e.sink.Emit(record); err != nil {
		return fmt.Errorf("write lifecycle event: %w", err)
	}
	return nil
}

func (e *eventEmitter) runtime(record event.Event) {
	if err := e.sink.Emit(record); err != nil {
		fmt.Fprintln(e.stderr, "write lifecycle event:", err)
		e.sink = event.DiscardSink{}
	}
}

func (e *eventEmitter) failure(phase string, err error) {
	fmt.Fprintln(e.stderr, err)
	e.runtime(event.Event{Name: event.Error, Phase: phase, Message: err.Error()})
}

func run(args []string, stdout, stderr io.Writer, buildVersion string, processLauncher launcher) int {
	emitter := eventEmitter{sink: event.DiscardSink{}, stderr: stderr}
	if cli.WantsJSONEvents(args) {
		emitter.sink = event.NewJSONSink(stdout)
	}

	cfg, err := cli.Parse(args, stderr)
	if err != nil {
		emitter.failure(event.PhaseArguments, err)
		return exitcode.Code(err)
	}
	if cfg.ShowVersion {
		if _, err := fmt.Fprintf(stdout, "samp-injector %s\n", buildVersion); err != nil {
			return fail(&emitter, event.PhaseRuntime, exitcode.PhaseInternal, fmt.Errorf("write version: %w", err))
		}
		return exitcode.Success
	}
	return runSession(cfg, &emitter, processLauncher)
}

func runSession(cfg cli.Config, emitter *eventEmitter, processLauncher launcher) int {
	session, err := processLauncher.Launch(cfg)
	if err != nil {
		return fail(emitter, event.PhaseLifecycle, exitcode.PhaseLifecycle, err)
	}
	defer func() { _ = session.Close() }()
	if err := emitter.startup(event.Event{Name: event.Launched}); err != nil {
		return fail(emitter, event.PhaseLifecycle, exitcode.PhaseInternal, err)
	}
	if cfg.WaitForModule {
		if err := emitter.startup(event.Event{Name: event.Waiting, Module: cfg.WaitModule}); err != nil {
			return fail(emitter, event.PhaseReadiness, exitcode.PhaseInternal, err)
		}
		if err := session.WaitForReadiness(cfg.WaitModule, cfg.WaitTimeout); err != nil {
			return fail(emitter, event.PhaseReadiness, exitcode.PhaseReadiness, err)
		}
	}
	if err := emitter.startup(event.Event{Name: event.Injecting}); err != nil {
		return fail(emitter, event.PhaseInjection, exitcode.PhaseInternal, err)
	}
	if err := session.Inject(cfg.DLLPath); err != nil {
		return fail(emitter, event.PhaseInjection, exitcode.PhaseInjection, err)
	}

	emitter.runtime(event.Event{Name: event.Injected})
	return waitForGame(session, emitter)
}

func waitForGame(session gameSession, emitter *eventEmitter) int {
	running, err := session.Running()
	if err != nil {
		return fail(emitter, event.PhaseRuntime, exitcode.PhaseInternal, err)
	}
	if running {
		if err := session.Wait(); err != nil {
			return fail(emitter, event.PhaseRuntime, exitcode.PhaseInternal, err)
		}
	}
	code, err := session.ExitCode()
	if err != nil {
		return fail(emitter, event.PhaseRuntime, exitcode.PhaseInternal, err)
	}
	gameCode := int(code)
	emitter.runtime(event.Event{Name: event.GameExited, Code: &gameCode})
	return gameCode
}

func fail(emitter *eventEmitter, eventPhase string, codePhase exitcode.Phase, err error) int {
	emitter.failure(eventPhase, err)
	return exitcode.Code(&exitcode.Error{Phase: codePhase, Err: err})
}
