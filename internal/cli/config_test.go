package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParse_usesGameDirectoryAndForwardsArguments(t *testing.T) {
	dir := t.TempDir()
	game := filepath.Join(dir, "gta sa.exe")
	dll := filepath.Join(dir, "samp.dll")
	if err := os.WriteFile(game, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dll, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{"--game", game, "--dll", dll, "--", "-n", "Player Name", ""}, new(bytes.Buffer))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkingDirectory != dir {
		t.Fatalf("working directory = %q, want %q", cfg.WorkingDirectory, dir)
	}
	if len(cfg.GameArgs) != 3 || cfg.GameArgs[1] != "Player Name" || cfg.GameArgs[2] != "" {
		t.Fatalf("forwarded args = %#v", cfg.GameArgs)
	}
	if cfg.WaitModule != DefaultWaitModule || cfg.WaitTimeout != 30*time.Second {
		t.Fatalf("defaults = %#v", cfg)
	}
}

func TestParse_rejectsInvalidReadinessOptions(t *testing.T) {
	dir := t.TempDir()
	game := filepath.Join(dir, "gta.exe")
	dll := filepath.Join(dir, "samp.dll")
	if err := os.WriteFile(game, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dll, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Parse([]string{"--game", game, "--dll", dll, "--wait-module", "dir/module.dll", "--no-wait-module"}, new(bytes.Buffer))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("error = %v, want ErrUsage", err)
	}
}

func TestParse_versionDoesNotRequireSessionPaths(t *testing.T) {
	cfg, err := Parse([]string{"--version"}, new(bytes.Buffer))
	if err != nil {
		t.Fatalf("Parse(--version) error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Error("Parse(--version) ShowVersion = false, want true")
	}
}

func TestParse_rejectsUnsupportedEventMode(t *testing.T) {
	_, err := Parse([]string{"--events=xml"}, new(bytes.Buffer))
	if !errors.Is(err, ErrUsage) {
		t.Errorf("Parse(--events=xml) error = %v, want ErrUsage", err)
	}
}

func TestWantsJSONEvents(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "equals", args: []string{"--events=json"}, want: true},
		{name: "separate", args: []string{"--events", "json"}, want: true},
		{name: "forwarded", args: []string{"--", "--events=json"}, want: false},
		{name: "absent", args: []string{"--game", "gta_sa.exe"}, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := WantsJSONEvents(test.args); got != test.want {
				t.Errorf("WantsJSONEvents(%q) = %t, want %t", test.args, got, test.want)
			}
		})
	}
}
