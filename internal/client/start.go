package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/elionaz/panal/internal/paths"
	"github.com/elionaz/panal/internal/transport"
)

// DaemonExecutable is the binary that should run `daemon` for the panal
// executable exe. The CLI bundled in panal.app (Contents/Helpers/panal,
// which Homebrew symlinks into PATH) defers to the app's own executable
// (Contents/MacOS/panal) when it exists and is executable: only a process
// of the bundle gets native notifications and their clicks. Any other exe
// is returned unchanged.
func DaemonExecutable(exe string) string {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	app := appExeForHelper(real)
	if app == "" {
		return exe
	}
	if fi, err := os.Stat(app); err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return exe
	}
	return app
}

// CLIExecutable is the binary to run CLI commands (`mcp`, `browser mcp`)
// for the panal executable exe. The app's own executable
// (X.app/Contents/MacOS/panal), which runs the daemon of an app install,
// only knows `daemon`, `notify` and `version`: anything else starts the
// app, which hands over to the open window and exits. So for it, this is
// the CLI bundled next to it (X.app/Contents/Helpers/panal) when that
// exists and is executable. Any other exe is returned unchanged.
func CLIExecutable(exe string) string {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	const marker = ".app/Contents/MacOS/"
	i := strings.LastIndex(real, marker)
	if i < 0 || strings.Contains(real[i+len(marker):], "/") || i+len(marker) == len(real) {
		return exe
	}
	cli := real[:i+len(".app")] + "/Contents/Helpers/panal"
	if fi, err := os.Stat(cli); err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return exe
	}
	return cli
}

// appExeForHelper maps "/A/X.app/Contents/Helpers/panal" to
// "/A/X.app/Contents/MacOS/panal"; "" when path is not a bundle helper.
func appExeForHelper(path string) string {
	const marker = ".app/Contents/Helpers/"
	i := strings.LastIndex(path, marker)
	if i < 0 || strings.Contains(path[i+len(marker):], "/") || i+len(marker) == len(path) {
		return ""
	}
	return path[:i+len(".app")] + "/Contents/MacOS/panal"
}

// StartDaemon launches `<exe> daemon` detached from the caller (new session,
// stdio to the daemon log) and waits until its socket accepts connections.
// exe is typically os.Executable() of the panal binary.
func StartDaemon(ctx context.Context, exe string, p paths.Paths) error {
	if err := p.Ensure(); err != nil {
		return err
	}
	logf, err := os.OpenFile(p.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	// Only what this daemon writes explains its exit (the log is shared
	// with earlier runs).
	logStart, _ := logf.Seek(0, io.SeekEnd)
	cmd := exec.Command(exe, "daemon")
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }() // reap if we outlive it

	t := transport.Default()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		if transport.Alive(t, p.Socket()) {
			return nil
		}
		select {
		case err := <-exited:
			if transport.Alive(t, p.Socket()) {
				return nil // another starter won the race
			}
			if why := startupError(p.LogFile(), logStart); why != "" {
				return fmt.Errorf("daemon exited during startup: %s (see %s)", why, p.LogFile())
			}
			return fmt.Errorf("daemon exited during startup (%v); see %s", err, p.LogFile())
		case <-ctx.Done():
			return fmt.Errorf("daemon did not come up: %w; see %s", ctx.Err(), p.LogFile())
		case <-tick.C:
		}
	}
}

// startupError is the reason a daemon that failed to start printed last in
// its log ("panal: <error>" from the CLI, "panal daemon: <error>"
// from the app binary; e.g. a file written by a newer panal),
// or "". Only what was written from offset from on is looked at.
func startupError(logFile string, from int64) string {
	f, err := os.Open(logFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	const tail = 4096
	if fi, err := f.Stat(); err == nil {
		_, _ = f.Seek(max(from, fi.Size()-tail), io.SeekStart)
	}
	data, _ := io.ReadAll(f)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	last := lines[len(lines)-1]
	for _, prefix := range []string{"panal daemon: ", "panal: "} {
		if why, ok := strings.CutPrefix(last, prefix); ok {
			return why
		}
	}
	return ""
}
