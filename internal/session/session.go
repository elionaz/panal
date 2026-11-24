// Package session owns a single PTY-backed process: it spawns it, records its
// output into a ring buffer, fans output out to attached subscribers and
// tracks its status.
package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/elionaz/panal/internal/paths"
	"github.com/elionaz/panal/internal/protocol"
	"github.com/elionaz/panal/internal/status"
	"github.com/elionaz/panal/internal/termsig"
)

// Environment variables injected into every session.
const (
	EnvSession = protocol.EnvSession
	EnvSock    = protocol.EnvSock
	EnvHome    = paths.EnvHome
)

// subscriberQueue is how many output chunks a subscriber may lag behind
// before it is disconnected.
const subscriberQueue = 256

// Options configures Spawn.
type Options struct {
	ID          string
	ProjectID   string
	ProjectDir  string
	ProjectName string
	ProfileID   string
	Cmd         string
	Args        []string
	// Launch, when set, is the argv actually executed instead of Cmd+Args
	// (a wrapper that runs a worktree setup first and then execs Cmd+Args):
	// Cmd and Args stay what the session shows and records.
	Launch   []string
	Env      []string // extra KEY=VALUE pairs appended to os.Environ()
	Cwd      string   // defaults to ProjectDir
	SockPath string   // exported as PANAL_SOCK
	// Home is exported as PANAL_HOME; empty removes any inherited value.
	// Clients prefer PANAL_HOME over PANAL_SOCK, so the daemon sets
	// both consistently (see client.ResolveSocket).
	Home       string
	Cols, Rows uint16
	RingSize   int
	// Scrollback seeds the ring buffer (a restored session's saved output,
	// replayed before anything the new process prints).
	Scrollback []byte
	// CreatedAt overrides the creation time (a restored session keeps its
	// original one). Zero means now.
	CreatedAt time.Time

	StatusMode status.Mode
	IdleAfter  time.Duration
	OnState    func(status.Change) // see status.Options.OnChange

	// AttentionPatterns are matched against the recent ANSI-free output
	// once it settles; a match raises a "question" attention.
	AttentionPatterns []*regexp.Regexp

	// StallAfter flags a hook-driven turn without output nor events as
	// stalled (0 disables it). See status.Options.StallAfter.
	StallAfter time.Duration
	// OnActivity is called (from the session's goroutines, without locks
	// held) when the activity line, progress or stall flag changed, at most
	// about twice per second, plus an output heartbeat every few seconds.
	// stallRaised is true when the session has just become stalled.
	OnActivity func(s *Session, stallRaised bool)
}

// Pattern matching: how long output must be quiet before the tail is
// checked, and how much of it is kept.
const (
	patternQuiet = 300 * time.Millisecond
	tailSize     = 2048
	// bellAfterKey: a bare BEL this soon after a key or terminal reply
	// (not Enter) is a reaction to it, not a notification.
	bellAfterKey = 500 * time.Millisecond
)

// Session is a running (or exited) PTY process. Identity fields are immutable
// after Spawn; dynamic state is read through methods.
type Session struct {
	ID          string
	ProjectID   string
	ProjectDir  string
	ProjectName string
	ProfileID   string
	Cmd         string
	Args        []string
	Env         []string
	Cwd         string
	CreatedAt   time.Time
	Pid         int

	cmd    *exec.Cmd
	ptmx   *os.File
	ring   *Ring
	status *status.Machine

	// outMu serializes ring writes, subscriber fan-out and Attach so a new
	// subscriber's snapshot and live stream never overlap or leave a gap.
	outMu sync.Mutex
	subs  map[*Subscriber]struct{}

	sizeMu     sync.Mutex
	cols, rows uint16

	sig      termsig.Parser // only used by readLoop
	patterns []*regexp.Regexp
	tailMu   sync.Mutex
	tail     []byte    // ANSI-free recent output, for patterns
	tailAt   time.Time // last append
	tailDirt bool      // appended since last pattern check
	// draft: what was typed last did not end with Enter (the agent's input
	// line may hold unsent text); inputAt: when it was typed.
	draft   bool
	lastIn  byte // the last input byte (for "\" Enter split across writes)
	inputAt time.Time
	matched string // last line that matched (debounce)

	act        tracker
	lines      lineTracker
	onActivity func(*Session, bool)
	stallFlip  atomic.Bool // the stalled flag flipped since the last emission
	emitMu     sync.Mutex  // serialises OnActivity calls

	// beepAt: when input that doesn't submit last went to the PTY
	// (UnixNano): a key or a terminal reply (focus, reports). A bell right
	// after it is the program beeping at it.
	beepAt atomic.Int64

	done     chan struct{}
	readDone chan struct{}
	killOnce sync.Once

	// suspended: restored without a process (see Restore). released is
	// set (under outMu) once its subscribers were let go.
	suspended bool
	released  bool
}

// Spawn starts the process on a new PTY.
func Spawn(o Options) (*Session, error) {
	if o.ID == "" || o.Cmd == "" {
		return nil, errors.New("session: ID and Cmd are required")
	}
	if o.Cols == 0 {
		o.Cols = 80
	}
	if o.Rows == 0 {
		o.Rows = 24
	}
	s := newSession(o)
	s.status = status.New(status.Options{Mode: o.StatusMode, IdleAfter: o.IdleAfter, OnChange: o.OnState,
		StallAfter: o.StallAfter, OnStall: func(bool) { s.stallFlip.Store(true) }})

	cmd := exec.Command(o.Cmd, o.Args...)
	if len(o.Launch) > 0 {
		cmd = exec.Command(o.Launch[0], o.Launch[1:]...)
	}
	cmd.Dir = s.Cwd
	cmd.Env = buildEnv(o)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: s.cols, Rows: s.rows})
	if err != nil {
		return nil, fmt.Errorf("spawn %s: %w", o.Cmd, err)
	}
	s.cmd, s.ptmx, s.Pid = cmd, ptmx, cmd.Process.Pid

	go s.readLoop()
	go s.waitLoop()
	go s.tickLoop()
	return s, nil
}

// Restore returns a suspended session: the identity and scrollback
// (o.Scrollback) of a session from a previous daemon run, without a
// process. Its state is status.Suspended; attaching replays the scrollback
// and then waits (no live output) until Release. Input is refused and Kill
// is a no-op. The daemon replaces it with a Spawn'ed session of the same
// ID to resume it.
func Restore(o Options) *Session {
	s := newSession(o)
	s.suspended = true
	s.status = status.New(status.Options{Mode: o.StatusMode, Suspended: true})
	close(s.readDone)
	close(s.done)
	return s
}

func newSession(o Options) *Session {
	if o.Cols == 0 {
		o.Cols = 80
	}
	if o.Rows == 0 {
		o.Rows = 24
	}
	if o.Cwd == "" {
		o.Cwd = o.ProjectDir
	}
	created := o.CreatedAt.UTC()
	if o.CreatedAt.IsZero() {
		created = time.Now().UTC()
	}
	ring := NewRing(o.RingSize)
	if len(o.Scrollback) > 0 {
		ring.Write(o.Scrollback)
	}
	return &Session{
		ID:          o.ID,
		ProjectID:   o.ProjectID,
		ProjectDir:  o.ProjectDir,
		ProjectName: o.ProjectName,
		ProfileID:   o.ProfileID,
		Cmd:         o.Cmd,
		Args:        append([]string(nil), o.Args...),
		Env:         append([]string(nil), o.Env...),
		Cwd:         o.Cwd,
		CreatedAt:   created,
		ring:        ring,
		subs:        make(map[*Subscriber]struct{}),
		cols:        o.Cols,
		rows:        o.Rows,
		done:        make(chan struct{}),
		readDone:    make(chan struct{}),
		patterns:    o.AttentionPatterns,
		onActivity:  o.OnActivity,
		// Hook-driven agents report what they do; their screen's last line
		// is UI chrome (Claude Code's status line, a HUD), never activity.
		act: tracker{hookSeen: o.StatusMode == status.ModeHooks},
	}
}

func buildEnv(o Options) []string {
	var env []string
	hasTerm := false
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, EnvHome+"=") {
			continue
		}
		if strings.HasPrefix(kv, "TERM=") {
			hasTerm = true
		}
		env = append(env, kv)
	}
	if !hasTerm {
		env = append(env, "TERM=xterm-256color")
	}
	env = append(env, o.Env...)
	env = append(env, EnvSession+"="+o.ID)
	if o.SockPath != "" {
		env = append(env, EnvSock+"="+o.SockPath)
	}
	if o.Home != "" {
		env = append(env, EnvHome+"="+o.Home)
	}
	return env
}

func (s *Session) readLoop() {
	defer close(s.readDone)
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.status.Output()
			s.act.output()
			s.scan(chunk)
			s.broadcast(chunk)
			s.refreshActivity(time.Now())
		}
		if err != nil {
			// EIO is the normal "slave closed" signal on Linux/macOS.
			return
		}
	}
}

// scan looks for terminal notifications (BEL, OSC 9/777/99) and feeds the
// pattern tail. The chunk itself is passed to subscribers untouched.
func (s *Session) scan(chunk []byte) {
	onText := func(b []byte) {
		s.lines.feed(b)
		if len(s.patterns) > 0 {
			s.appendTail(b)
		}
	}
	s.sig.Feed(chunk, func(sg termsig.Signal) {
		if sg.Progress != nil {
			// Progress bars are information, never a request for attention.
			s.act.setProgress(*sg.Progress)
			return
		}
		if sg.Bell && time.Since(time.Unix(0, s.beepAt.Load())) < bellAfterKey {
			// The program beeping at what it was just sent (an unbound key,
			// a failed completion, focus reports left on by an editor), not
			// a request for attention.
			return
		}
		s.status.Signal(status.KindQuestion, sg.Text())
	}, onText)
}

// refreshActivity recomputes the output-derived activity line (rate
// limited) and emits a coalesced activity notification when one is due.
func (s *Session) refreshActivity(now time.Time) {
	if s.act.evalDue(now) {
		if line, ok := s.lines.candidate(); ok {
			s.act.setOutput(line, now)
		}
	}
	s.emitActivity(now)
}

func (s *Session) emitActivity(now time.Time) {
	if s.onActivity == nil {
		return
	}
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	flip := s.stallFlip.Swap(false)
	if !s.act.due(now, flip) {
		return
	}
	s.onActivity(s, flip && s.status.Stalled())
}

func (s *Session) appendTail(b []byte) {
	s.tailMu.Lock()
	defer s.tailMu.Unlock()
	for _, c := range b {
		if c == '\b' {
			s.tail = eraseLast(s.tail)
		} else {
			s.tail = append(s.tail, c)
		}
	}
	if over := len(s.tail) - tailSize; over > 0 {
		s.tail = append(s.tail[:0], s.tail[over:]...)
	}
	s.tailAt = time.Now()
	s.tailDirt = true
}

// checkPatterns runs once output has been quiet for patternQuiet. A line
// that already matched does not fire again until the user types.
func (s *Session) checkPatterns() {
	if len(s.patterns) == 0 {
		return
	}
	s.tailMu.Lock()
	if !s.tailDirt || time.Since(s.tailAt) < patternQuiet {
		s.tailMu.Unlock()
		return
	}
	s.tailDirt = false
	text := string(s.tail)
	var line string
	for _, re := range s.patterns {
		if loc := re.FindStringIndex(text); loc != nil {
			line = lineAround(text, loc[0], loc[1])
			break
		}
	}
	fire := line != "" && line != s.matched
	if fire {
		s.matched = line
	}
	s.tailMu.Unlock()
	if fire {
		s.status.Signal(status.KindQuestion, line)
	}
}

// lineAround returns the trimmed line(s) containing text[i:j].
func lineAround(text string, i, j int) string {
	start := strings.LastIndexAny(text[:i], "\r\n") + 1
	end := len(text)
	if k := strings.IndexAny(text[j:], "\r\n"); k >= 0 {
		end = j + k
	}
	line := strings.Join(strings.Fields(text[start:end]), " ")
	if r := []rune(line); len(r) > 200 {
		line = string(r[:200])
	}
	return line
}

func (s *Session) broadcast(chunk []byte) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.ring.Write(chunk)
	for sub := range s.subs {
		select {
		case sub.ch <- chunk:
		default:
			// Too slow: drop the subscriber rather than block the PTY.
			sub.dropped = true
			delete(s.subs, sub)
			close(sub.ch)
		}
	}
}

func (s *Session) waitLoop() {
	err := s.cmd.Wait()
	code := exitCode(s.cmd.ProcessState, err)
	// Drain whatever output is still buffered, but don't hang forever if a
	// grandchild keeps the slave side open.
	select {
	case <-s.readDone:
	case <-time.After(500 * time.Millisecond):
	}
	s.status.Exit(code)
	s.ptmx.Close()
	<-s.readDone

	s.outMu.Lock()
	for sub := range s.subs {
		delete(s.subs, sub)
		close(sub.ch)
	}
	s.outMu.Unlock()
	close(s.done)
}

func exitCode(ps *os.ProcessState, err error) int {
	if ps == nil {
		if err != nil {
			return -1
		}
		return 0
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

func (s *Session) tickLoop() {
	interval := min(max(s.status.IdleAfter()/4, 50*time.Millisecond), activityInterval/2)
	if len(s.patterns) > 0 {
		interval = min(interval, patternQuiet/2)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			s.status.Tick()
			s.checkPatterns()
			s.refreshActivity(time.Now())
		}
	}
}

// Subscriber receives live output chunks. C is closed when the session exits
// or when the subscriber falls too far behind (Dropped reports which).
type Subscriber struct {
	C       <-chan []byte
	ch      chan []byte
	s       *Session
	dropped bool // guarded by s.outMu
}

// Dropped reports whether the subscriber was disconnected for being slow.
// Only meaningful after C is closed.
func (sub *Subscriber) Dropped() bool {
	sub.s.outMu.Lock()
	defer sub.s.outMu.Unlock()
	return sub.dropped
}

// Close detaches the subscriber. It is safe to call multiple times.
func (sub *Subscriber) Close() {
	sub.s.outMu.Lock()
	defer sub.s.outMu.Unlock()
	if _, ok := sub.s.subs[sub]; ok {
		delete(sub.s.subs, sub)
		close(sub.ch)
	}
}

// Attach returns the current scrollback and a subscriber for output produced
// after it, atomically. Attaching to an exited session returns a subscriber
// whose channel is already closed.
func (s *Session) Attach() (snapshot []byte, sub *Subscriber) {
	ch := make(chan []byte, subscriberQueue)
	sub = &Subscriber{C: ch, ch: ch, s: s}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	snapshot = s.ring.Snapshot()
	if s.suspended {
		// Nothing will be written, but the stream stays open until the
		// session is resumed or discarded (Release).
		if s.released {
			close(ch)
		} else {
			s.subs[sub] = struct{}{}
		}
		return snapshot, sub
	}
	select {
	case <-s.done:
		close(ch)
	default:
		s.subs[sub] = struct{}{}
	}
	return snapshot, sub
}

// Alive reports whether the session has a running process (it is neither
// exited nor suspended).
func (s *Session) Alive() bool {
	if s.suspended {
		return false
	}
	_, exited := s.ExitCode()
	return !exited
}

// Suspended reports whether this is a restored session without a process
// (see Restore).
func (s *Session) Suspended() bool { return s.suspended }

// Release disconnects the subscribers of a suspended session (it was
// resumed or discarded); later Attach calls get a closed channel. It is a
// no-op for spawned sessions.
func (s *Session) Release() {
	if !s.suspended {
		return
	}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.released = true
	for sub := range s.subs {
		delete(s.subs, sub)
		close(sub.ch)
	}
}

// Scrollback returns a copy of the ring buffer.
func (s *Session) Scrollback() []byte { return s.ring.Snapshot() }

// OutputTotal is how many bytes were ever written to the scrollback (a
// cheap "changed since" check for persisting it).
func (s *Session) OutputTotal() int64 { return s.ring.Total() }

// RingCap is the scrollback capacity.
func (s *Session) RingCap() int { return s.ring.Cap() }

// Draft reports whether the input line may hold unsent text (what was last
// typed was not a submit, see submits), and when input was last typed.
func (s *Session) Draft() (draft bool, at time.Time) {
	s.tailMu.Lock()
	defer s.tailMu.Unlock()
	return s.draft, s.inputAt
}

// submits reports whether input p, typed after a byte prev, leaves an
// agent's input line empty: a lone Enter (CR), or Ctrl-C, which clears it.
// LF (Shift+Enter in the app, Ctrl-J), Option+Enter (ESC CR) and "\" Enter
// add a line to the draft instead. Anything else leaves a draft. Erring on
// "draft" only holds automatic delivery back.
func submits(prev byte, p []byte) bool {
	if len(p) == 0 {
		return false
	}
	last := p[len(p)-1]
	if last == 0x03 {
		return true
	}
	if last != '\r' {
		return false
	}
	before := prev
	if len(p) > 1 {
		before = p[len(p)-2]
	}
	return before != 0x1b && before != '\\'
}

// Write sends input to the PTY. Input that is not just an automatic
// terminal reply counts as the user answering: it acknowledges attention.
func (s *Session) Write(p []byte) (int, error) {
	if s.suspended {
		return 0, errors.New("session suspended")
	}
	select {
	case <-s.done:
		return 0, errors.New("session exited")
	default:
	}
	beep := true // a bell soon after this is a beep at it
	if !isTerminalReply(p) {
		s.status.Input()
		s.tailMu.Lock()
		s.tail, s.matched, s.tailDirt = s.tail[:0], "", false
		if len(p) > 0 {
			s.draft = !submits(s.lastIn, p)
			s.lastIn = p[len(p)-1]
			s.inputAt = time.Now()
			// After a line ends (or Ctrl-C) the bell is the command's own.
			last := p[len(p)-1]
			beep = last != '\r' && last != '\n' && last != 0x03
		}
		s.tailMu.Unlock()
	}
	if beep {
		s.beepAt.Store(time.Now().UnixNano())
	} else {
		s.beepAt.Store(0)
	}
	return s.ptmx.Write(p)
}

// isTerminalReply reports whether p consists only of sequences a terminal
// emulator sends on its own: device attribute / status / mode reports
// (CSI … c|n|R|y|t|u), focus in/out (CSI I|O), and OSC/DCS replies.
func isTerminalReply(p []byte) bool {
	if len(p) == 0 {
		return true
	}
	for len(p) > 0 {
		if len(p) < 2 || p[0] != 0x1b {
			return false
		}
		switch p[1] {
		case '[':
			i := 2
			for i < len(p) && p[i] >= 0x20 && p[i] <= 0x3f {
				i++
			}
			if i >= len(p) {
				return false
			}
			switch p[i] {
			case 'c', 'n', 'R', 'y', 't', 'u', 'I', 'O':
			default:
				return false
			}
			p = p[i+1:]
		case ']', 'P':
			// Ends at BEL or ST.
			end := -1
			for i := 2; i < len(p); i++ {
				if p[i] == 0x07 {
					end = i + 1
					break
				}
				if p[i] == 0x1b && i+1 < len(p) && p[i+1] == '\\' {
					end = i + 2
					break
				}
			}
			if end < 0 {
				return false
			}
			p = p[end:]
		default:
			return false
		}
	}
	return true
}

// Resize changes the PTY window size.
func (s *Session) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return fmt.Errorf("invalid size %dx%d", cols, rows)
	}
	if s.suspended {
		// Remembered for the resumed process.
		s.sizeMu.Lock()
		s.cols, s.rows = cols, rows
		s.sizeMu.Unlock()
		return nil
	}
	select {
	case <-s.done:
		return errors.New("session exited")
	default:
	}
	s.sizeMu.Lock()
	defer s.sizeMu.Unlock()
	if err := pty.Setsize(s.ptmx, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		return err
	}
	s.cols, s.rows = cols, rows
	return nil
}

// Size returns the last applied window size.
func (s *Session) Size() (cols, rows uint16) {
	s.sizeMu.Lock()
	defer s.sizeMu.Unlock()
	return s.cols, s.rows
}

// Notify applies an explicit status event.
func (s *Session) Notify(st status.State) error { return s.status.Notify(st) }

// NotifyEvent applies an explicit status event with attention details and
// optional activity.
func (s *Session) NotifyEvent(ev status.Event) error {
	if ev.Activity != "" {
		s.act.setHook(ev.Activity, ev.ActivityKind, ev.ActivityTarget, time.Now())
	}
	err := s.status.NotifyEvent(ev)
	if ev.Turn {
		s.act.touch() // the turn timer restarted
	}
	s.emitActivity(time.Now())
	return err
}

// SetActivity records activity reported explicitly (hook source) without
// touching the state.
func (s *Session) SetActivity(text, kind, target string) {
	if text != "" && s.act.setHook(text, kind, target, time.Now()) {
		s.emitActivity(time.Now())
	}
}

// Live returns the session's activity line and progress.
func (s *Session) Live() Live { return s.act.snapshot() }

// Timers returns the session's activity timers and stall flag.
func (s *Session) Timers() status.Timers { return s.status.Timers() }

// Tail returns the last n lines of output as plain text (see TailLines).
func (s *Session) Tail(n int) []string { return TailLines(s.ring.Tail(tailBytes), n) }

// Ack marks the session's attention as seen.
func (s *Session) Ack() bool { return s.status.Ack() }

// Attention returns the session's current attention.
func (s *Session) Attention() status.Attention { return s.status.Attention() }

// State returns the current status.
func (s *Session) State() status.State { return s.status.State() }

// ExitCode returns the exit code and whether the process has exited.
func (s *Session) ExitCode() (int, bool) { return s.status.ExitCode() }

// IdleSource is what made the session idle last (status.Machine.IdleSource).
func (s *Session) IdleSource() status.Source { return s.status.IdleSource() }

// Done is closed once the process has exited and all output was delivered.
func (s *Session) Done() <-chan struct{} { return s.done }

// Kill terminates the process group: SIGHUP+SIGTERM, then SIGKILL after
// grace. It returns once the process has exited.
func (s *Session) Kill(grace time.Duration) {
	if s.cmd == nil {
		return // suspended: no process (and pid 0 would mean our own group)
	}
	s.killOnce.Do(func() {
		pgid := s.Pid // pty.Start makes the child a session (and group) leader
		_ = syscall.Kill(-pgid, syscall.SIGHUP)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-s.done:
			return
		case <-time.After(grace):
		}
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})
	<-s.done
}
