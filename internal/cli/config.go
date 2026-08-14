// Package cli parses and validates samp-injector command-line arguments.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultWaitModule is the startup module used for the default readiness wait.
const DefaultWaitModule = "vorbisFile.dll"

// ErrUsage identifies invalid command-line input.
var ErrUsage = errors.New("invalid command line")

// Config contains the validated inputs for one injector session.
type Config struct {
	GamePath         string
	DLLPath          string
	WorkingDirectory string
	WaitModule       string
	WaitForModule    bool
	WaitTimeout      time.Duration
	GameArgs         []string
	EventsJSON       bool
	ShowVersion      bool
}

// Parse validates injector flags and preserves arguments after -- for GTA.
func Parse(args []string, stderr io.Writer) (Config, error) {
	flags, forwarded := splitForwarded(args)
	set := flag.NewFlagSet("samp-injector", flag.ContinueOnError)
	set.SetOutput(stderr)
	game := set.String("game", "", "GTA executable to launch")
	dll := set.String("dll", "", "DLL to inject")
	cwd := set.String("cwd", "", "working directory")
	waitModule := set.String("wait-module", DefaultWaitModule, "module to wait for")
	noWait := set.Bool("no-wait-module", false, "inject without waiting for a module")
	waitTimeout := set.Duration("wait-timeout", 30*time.Second, "readiness wait timeout")
	events := set.String("events", "", "machine-readable events (json)")
	showVersion := set.Bool("version", false, "print version and exit")

	if err := set.Parse(flags); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrUsage, err)
	}
	if set.NArg() != 0 {
		return Config{}, fmt.Errorf("%w: unexpected argument %q", ErrUsage, set.Arg(0))
	}
	if *events != "" && *events != "json" {
		return Config{}, fmt.Errorf("%w: --events must be json", ErrUsage)
	}
	if *showVersion {
		if *events != "" {
			return Config{}, fmt.Errorf("%w: --version and --events cannot be combined", ErrUsage)
		}
		return Config{ShowVersion: true}, nil
	}
	if *game == "" || *dll == "" {
		return Config{}, fmt.Errorf("%w: --game and --dll are required", ErrUsage)
	}
	if *waitTimeout <= 0 {
		return Config{}, fmt.Errorf("%w: --wait-timeout must be greater than zero", ErrUsage)
	}
	if *noWait && *waitModule != DefaultWaitModule {
		return Config{}, fmt.Errorf("%w: --wait-module and --no-wait-module cannot be combined", ErrUsage)
	}
	if *noWait {
		*waitModule = ""
	} else if filepath.Base(*waitModule) != *waitModule || strings.TrimSpace(*waitModule) == "" {
		return Config{}, fmt.Errorf("%w: --wait-module must be a filename", ErrUsage)
	}

	gamePath, err := existingFile(*game)
	if err != nil {
		return Config{}, fmt.Errorf("%w: game: %w", ErrUsage, err)
	}
	dllPath, err := existingFile(*dll)
	if err != nil {
		return Config{}, fmt.Errorf("%w: dll: %w", ErrUsage, err)
	}
	workingDirectory := *cwd
	if workingDirectory == "" {
		workingDirectory = filepath.Dir(gamePath)
	}
	workingDirectory, err = filepath.Abs(workingDirectory)
	if err != nil {
		return Config{}, fmt.Errorf("%w: cwd: %w", ErrUsage, err)
	}
	info, err := os.Stat(workingDirectory)
	if err != nil {
		return Config{}, fmt.Errorf("%w: cwd: %w", ErrUsage, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("%w: cwd is not a directory: %s", ErrUsage, workingDirectory)
	}
	return Config{GamePath: gamePath, DLLPath: dllPath, WorkingDirectory: workingDirectory, WaitModule: *waitModule, WaitForModule: !*noWait, WaitTimeout: *waitTimeout, GameArgs: append([]string(nil), forwarded...), EventsJSON: *events == "json"}, nil
}

// WantsJSONEvents reports whether args explicitly request the JSON event stream.
func WantsJSONEvents(args []string) bool {
	for i, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--events=json" || arg == "-events=json" {
			return true
		}
		if (arg == "--events" || arg == "-events") && i+1 < len(args) {
			return args[i+1] == "json"
		}
	}
	return false
}

func splitForwarded(args []string) ([]string, []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func existingFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("path is a directory")
	}
	return filepath.Clean(abs), nil
}
