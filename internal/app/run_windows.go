//go:build windows

package app

import (
	"io"
	"time"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/cli"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/win32"
)

// Run launches, injects, and owns GTA for the duration of the session.
func Run(args []string, stdout, stderr io.Writer, buildVersion string) int {
	return run(args, stdout, stderr, buildVersion, windowsLauncher{})
}

type windowsLauncher struct{}

func (windowsLauncher) Launch(cfg cli.Config) (gameSession, error) {
	process, err := win32.Launch(cfg.GamePath, cfg.WorkingDirectory, cfg.GameArgs)
	if err != nil {
		return nil, err
	}
	return &windowsSession{process: process}, nil
}

type windowsSession struct {
	process win32.Process
}

func (s *windowsSession) Close() error { return s.process.Close() }

func (s *windowsSession) WaitForReadiness(module string, timeout time.Duration) error {
	return win32.WaitForModule(s.process.Handle, module, timeout)
}

func (s *windowsSession) Inject(path string) error {
	return win32.Inject(s.process.Handle, path)
}

func (s *windowsSession) Running() (bool, error) { return s.process.Running() }

func (s *windowsSession) Wait() error {
	_, err := s.process.Wait(-1)
	return err
}

func (s *windowsSession) ExitCode() (uint32, error) { return s.process.ExitCode() }
