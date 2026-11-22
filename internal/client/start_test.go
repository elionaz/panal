package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppExeForHelper(t *testing.T) {
	for in, want := range map[string]string{
		"/Applications/panal.app/Contents/Helpers/panal":     "/Applications/panal.app/Contents/MacOS/panal",
		"/x/My Apps/Dev.app/Contents/Helpers/panal":          "/x/My Apps/Dev.app/Contents/MacOS/panal",
		"/a.app/b/c.app/Contents/Helpers/panal":              "/a.app/b/c.app/Contents/MacOS/panal",
		"/Applications/panal.app/Contents/MacOS/panal":       "",
		"/Applications/panal.app/Contents/Helpers/sub/panal": "",
		"/Applications/panal.app/Contents/Helpers/":          "",
		"/opt/homebrew/bin/panal":                            "",
		"/Users/me/go/bin/panal":                             "",
	} {
		if got := appExeForHelper(in); got != want {
			t.Errorf("appExeForHelper(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDaemonExecutable(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(p string, mode os.FileMode) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	helper := write(filepath.Join(dir, "panal.app/Contents/Helpers/panal"), 0o755)
	appExe := write(filepath.Join(dir, "panal.app/Contents/MacOS/panal"), 0o755)

	// The bundled CLI, directly and through a symlink chain (Homebrew's
	// PATH link -> Caskroom link -> Helpers/panal).
	link1 := filepath.Join(dir, "Caskroom", "panal")
	link2 := filepath.Join(dir, "bin", "panal")
	os.MkdirAll(filepath.Dir(link1), 0o755)
	os.MkdirAll(filepath.Dir(link2), 0o755)
	if err := os.Symlink(helper, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{helper, link1, link2} {
		if got := DaemonExecutable(exe); got != appExe {
			t.Errorf("DaemonExecutable(%q) = %q, want %q", exe, got, appExe)
		}
	}

	// Outside a bundle (and for the app's own binary): unchanged.
	plain := write(filepath.Join(dir, "go/bin/panal"), 0o755)
	for _, exe := range []string{plain, appExe, filepath.Join(dir, "missing")} {
		if got := DaemonExecutable(exe); got != exe {
			t.Errorf("DaemonExecutable(%q) = %q, want unchanged", exe, got)
		}
	}

	// A bundle whose app binary is missing or not executable: unchanged.
	lone := write(filepath.Join(dir, "Lone.app/Contents/Helpers/panal"), 0o755)
	if got := DaemonExecutable(lone); got != lone {
		t.Errorf("no app binary: got %q", got)
	}
	noexec := write(filepath.Join(dir, "NoExec.app/Contents/Helpers/panal"), 0o755)
	write(filepath.Join(dir, "NoExec.app/Contents/MacOS/panal"), 0o644)
	if got := DaemonExecutable(noexec); got != noexec {
		t.Errorf("non-executable app binary: got %q", got)
	}
}

func TestStartupError(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "daemon.log")
	for in, want := range map[string]string{
		"time=x level=INFO msg=start\npanal: projects.json was written by a newer panal (format 2; this build reads up to 1): update panal\n": "projects.json was written by a newer panal (format 2; this build reads up to 1): update panal",
		"panal daemon: listen unix /x/d.sock: bind: invalid argument\n":                                                                       "listen unix /x/d.sock: bind: invalid argument",
		"time=x level=INFO msg=start\n": "",
		"":                              "",
	} {
		if err := os.WriteFile(log, []byte(in), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := startupError(log, 0); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if got := startupError(filepath.Join(dir, "missing.log"), 0); got != "" {
		t.Errorf("missing log: %q", got)
	}
	// An earlier run's error is not this one's.
	old := "panal daemon: an old failure\n"
	if err := os.WriteFile(log, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := startupError(log, int64(len(old))); got != "" {
		t.Errorf("stale reason: %q", got)
	}
}

func TestCLIExecutable(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mk := func(p string, mode os.FileMode) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	app := mk(filepath.Join(dir, "panal.app/Contents/MacOS/panal"), 0o755)
	// No bundled CLI: the app's executable stays.
	if got := CLIExecutable(app); got != app {
		t.Fatalf("without Helpers = %q", got)
	}
	cli := mk(filepath.Join(dir, "panal.app/Contents/Helpers/panal"), 0o755)
	if got := CLIExecutable(app); got != cli {
		t.Fatalf("app exe = %q, want %q", got, cli)
	}
	// The CLI itself, and binaries outside a bundle, stay as they are.
	for _, exe := range []string{cli, mk(filepath.Join(dir, "bin/panal"), 0o755)} {
		if got := CLIExecutable(exe); got != exe {
			t.Fatalf("CLIExecutable(%q) = %q", exe, got)
		}
	}
	// A bundled CLI that cannot run is not used.
	os.Chmod(cli, 0o644)
	if got := CLIExecutable(app); got != app {
		t.Fatalf("non-executable Helpers = %q", got)
	}
}
