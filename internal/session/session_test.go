package session

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/elionaz/panal/internal/status"
)

func spawn(t *testing.T, o Options) *Session {
	t.Helper()
	if o.ID == "" {
		o.ID = "test"
	}
	if o.Cwd == "" {
		o.Cwd = t.TempDir()
	}
	s, err := Spawn(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Kill(time.Second) })
	return s
}

func waitDone(t *testing.T, s *Session) {
	t.Helper()
	waitDoneWithin(t, s, 5*time.Second)
}

func waitDoneWithin(t *testing.T, s *Session, d time.Duration) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(d):
		t.Fatal("session did not exit")
	}
}

func TestSpawnEnvAndExitCode(t *testing.T) {
	s := spawn(t, Options{ID: "abc", Cmd: "/bin/sh", SockPath: "/tmp/x.sock",
		Args: []string{"-c", `echo "id=$PANAL_SESSION sock=$PANAL_SOCK foo=$FOO"; pwd; exit 7`},
		Env:  []string{"FOO=bar"}})
	waitDone(t, s)
	out := string(s.Scrollback())
	if !strings.Contains(out, "id=abc sock=/tmp/x.sock foo=bar") {
		t.Fatalf("env not injected: %q", out)
	}
	if !strings.Contains(out, s.Cwd) && !strings.Contains(out, "/private"+s.Cwd) {
		t.Fatalf("cwd not applied: %q (want %s)", out, s.Cwd)
	}
	code, exited := s.ExitCode()
	if !exited || code != 7 || s.State() != status.Exited {
		t.Fatalf("code=%d exited=%v state=%s", code, exited, s.State())
	}
	if _, err := s.Write([]byte("x")); err == nil {
		t.Fatal("write after exit should fail")
	}
}

func TestAttachLiveAndResize(t *testing.T) {
	s := spawn(t, Options{Cmd: "/bin/cat"})
	snap, sub := s.Attach()
	defer sub.Close()
	if len(snap) != 0 {
		t.Fatalf("unexpected snapshot %q", snap)
	}
	if err := s.Resize(100, 40); err != nil {
		t.Fatal(err)
	}
	if c, r := s.Size(); c != 100 || r != 40 {
		t.Fatalf("size %dx%d", c, r)
	}
	s.Write([]byte("ping\n"))
	var got bytes.Buffer
	deadline := time.After(3 * time.Second)
	for strings.Count(got.String(), "ping") < 2 {
		select {
		case c := <-sub.C:
			got.Write(c)
		case <-deadline:
			t.Fatalf("timeout, got %q", got.String())
		}
	}
	s.Kill(time.Second)
	if _, ok := <-sub.C; ok {
		// drain remaining
		for range sub.C {
		}
	}
	if sub.Dropped() {
		t.Fatal("subscriber should not be marked dropped on exit")
	}
	// Attaching after exit still yields scrollback and a closed channel.
	snap, sub2 := s.Attach()
	if !bytes.Contains(snap, []byte("ping")) {
		t.Fatalf("scrollback missing: %q", snap)
	}
	if _, ok := <-sub2.C; ok {
		t.Fatal("expected closed channel")
	}
}

func TestSlowSubscriberDropped(t *testing.T) {
	s := spawn(t, Options{Cmd: "/bin/sh", Args: []string{"-c", "yes | head -c 2000000"}, RingSize: 1024})
	_, slow := s.Attach() // never read
	// 2 MB through the PTY (enough to overflow the queue with Linux's 4 KB
	// reads) takes 0.7s here but over 5s on a busy macOS CI runner under
	// -race: a generous limit, a hang still fails.
	waitDoneWithin(t, s, 30*time.Second)
	for range slow.C {
	}
	if !slow.Dropped() {
		t.Fatal("slow subscriber should have been dropped")
	}
	if got := len(s.Scrollback()); got != 1024 {
		t.Fatalf("ring len = %d", got)
	}
}

func TestKillInteractiveShell(t *testing.T) {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	s := spawn(t, Options{Cmd: sh, Args: []string{"-i"}})
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	s.Kill(2 * time.Second)
	if s.State() != status.Exited {
		t.Fatalf("state %s", s.State())
	}
	t.Logf("killed in %s", time.Since(start))
}

func TestAttentionPatterns(t *testing.T) {
	changes := make(chan status.Change, 16)
	s := spawn(t, Options{Cmd: "/bin/sh", Args: []string{"-c", `printf '\033[1mDo you want to proceed?\033[0m (y/n) '; read a; printf 'ok\n'; sleep 5`},
		IdleAfter:         time.Hour,
		AttentionPatterns: []*regexp.Regexp{regexp.MustCompile(`proceed\?`)},
		OnState:           func(c status.Change) { changes <- c },
	})
	deadline := time.After(5 * time.Second)
	for {
		select {
		case c := <-changes:
			if !c.Raised {
				continue
			}
			if c.To != status.WaitingInput || c.Attention.Message != "Do you want to proceed? (y/n)" {
				t.Fatalf("bad change %+v", c)
			}
			// Answering resumes and lets the same prompt fire again later.
			s.Write([]byte("y\n"))
			if s.State() != status.Running {
				t.Fatalf("state after input %s", s.State())
			}
			return
		case <-deadline:
			t.Fatal("pattern did not raise attention")
		}
	}
}

// A bell right after something reached the PTY (here a focus report the
// program doesn't understand, as zsh with focus reporting left on by vim)
// is a beep at that input; a bell later on asks for attention.
func TestBellRightAfterInputIsNoQuestion(t *testing.T) {
	changes := make(chan status.Change, 16)
	s := spawn(t, Options{Cmd: "/bin/sh", Args: []string{"-c", `stty raw -echo; printf 'ready\n'; dd bs=1 count=3 2>/dev/null >/dev/null; printf '\a'; sleep 1; printf 'done\a'; sleep 5`},
		IdleAfter: time.Hour,
		OnState:   func(c status.Change) { changes <- c },
	})
	snap, sub := s.Attach()
	defer sub.Close()
	got := string(snap)
	for wait := time.After(5 * time.Second); !strings.Contains(got, "ready"); {
		select {
		case c := <-sub.C:
			got += string(c)
		case <-wait:
			t.Fatalf("no prompt, got %q", got)
		}
	}
	wrote := time.Now()
	s.Write([]byte("\x1b[I"))
	deadline := time.After(5 * time.Second)
	for {
		select {
		case c := <-changes:
			if !c.Raised {
				continue
			}
			if since := time.Since(wrote); since < 900*time.Millisecond {
				t.Fatalf("the beep at the input raised %s after it: %+v", since, c)
			}
			return
		case <-deadline:
			t.Fatal("the later bell did not raise attention")
		}
	}
}

func TestIsTerminalReply(t *testing.T) {
	for in, want := range map[string]bool{
		"\x1b[I": true, "\x1b[O": true, "\x1b[?1;2c": true, "\x1b[12;5R": true, "\x1b]11;rgb:1414/1515/1818\x1b\\": true,
		"y": false, "\r": false, "\x1b[A": false, "\x1b[Ix": false,
	} {
		if got := isTerminalReply([]byte(in)); got != want {
			t.Errorf("isTerminalReply(%q) = %v", in, got)
		}
	}
}

func TestRestoreSuspended(t *testing.T) {
	s := Restore(Options{ID: "r1", Cmd: "/bin/sh", Cwd: "/somewhere", RingSize: 16,
		Scrollback: []byte("0123456789abcdefXYZ"), CreatedAt: time.Unix(1_700_000_000, 0)})
	if !s.Suspended() || s.Alive() || s.State() != status.Suspended || s.Pid != 0 {
		t.Fatalf("restored: suspended=%v alive=%v state=%s pid=%d", s.Suspended(), s.Alive(), s.State(), s.Pid)
	}
	if got := string(s.Scrollback()); got != "3456789abcdefXYZ" { // capped to the ring
		t.Fatalf("scrollback %q", got)
	}
	if !s.CreatedAt.Equal(time.Unix(1_700_000_000, 0)) {
		t.Fatalf("created %v", s.CreatedAt)
	}
	if _, err := s.Write([]byte("x")); err == nil {
		t.Fatal("write accepted")
	}
	if err := s.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	if c, r := s.Size(); c != 120 || r != 40 {
		t.Fatalf("size %dx%d", c, r)
	}
	s.Kill(time.Millisecond) // no process: returns at once, signals nobody

	snap, sub := s.Attach()
	if string(snap) != "3456789abcdefXYZ" {
		t.Fatalf("snapshot %q", snap)
	}
	select {
	case <-sub.C:
		t.Fatal("stream closed before Release")
	case <-time.After(50 * time.Millisecond):
	}
	s.Release()
	if _, ok := <-sub.C; ok {
		t.Fatal("chunk after Release")
	}
	if _, sub2 := s.Attach(); sub2 != nil {
		if _, ok := <-sub2.C; ok {
			t.Fatal("attach after Release stays open")
		}
	}
}

func TestSpawnSeededScrollback(t *testing.T) {
	s := spawn(t, Options{Cmd: "/bin/sh", Args: []string{"-c", "echo new"}, Scrollback: []byte("old|")})
	waitDone(t, s)
	if out := string(s.Scrollback()); !strings.HasPrefix(out, "old|") || !strings.Contains(out, "new") {
		t.Fatalf("scrollback %q", out)
	}
}

func TestSubmits(t *testing.T) {
	for _, c := range []struct {
		prev byte
		in   string
		want bool
	}{
		{0, "\r", true},
		{'a', "\r", true},
		{0, "hello\r", true},
		{0, "\x03", true},
		{0, "half", false},
		{0, "\n", false},     // Shift+Enter in the app
		{0, "line\n", false}, // Ctrl-J
		{0, "\x1b\r", false}, // Option+Enter
		{'\\', "\r", false},  // "\" then Enter, two writes
		{0, "x\\\r", false},  // "\" Enter in one write
		{0, "\x1b[A", false}, // an arrow: maybe a recalled prompt
		{0, "", false},
	} {
		if got := submits(c.prev, []byte(c.in)); got != c.want {
			t.Errorf("submits(%q, %q) = %v, want %v", c.prev, c.in, got, c.want)
		}
	}
}

// Hook-driven sessions never take their screen's last line as activity: for
// Claude Code that is its status line (a HUD like oh-my-claudecode's).
func TestHookSessionsIgnoreOutputActivity(t *testing.T) {
	hud := `printf '[OMC#4.13.5] | 5h:6%% session:143m ctx:15%%\n'; sleep 5`
	heur := spawn(t, Options{ID: "heur", Cmd: "/bin/sh", Args: []string{"-c", hud}, StatusMode: status.ModeHeuristic, IdleAfter: time.Hour})
	hooks := spawn(t, Options{ID: "hooks", Cmd: "/bin/sh", Args: []string{"-c", hud}, StatusMode: status.ModeHooks, IdleAfter: time.Hour})
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(heur.Live().Activity, "OMC") {
		if time.Now().After(deadline) {
			t.Fatalf("heuristic session: activity %q, want its output line", heur.Live().Activity)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if a := hooks.Live().Activity; a != "" {
		t.Fatalf("hook session took output as activity: %q", a)
	}
	hooks.SetActivity("Bash: go test", "tool", "go test")
	if a := hooks.Live(); a.Activity != "Bash: go test" || a.ActivitySource != ActivityHook {
		t.Fatalf("hook activity: %+v", a)
	}
}
