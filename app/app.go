package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/elionaz/panal/internal/client"
	"github.com/elionaz/panal/internal/forge"
	"github.com/elionaz/panal/internal/gitx"
	"github.com/elionaz/panal/internal/history"
	"github.com/elionaz/panal/internal/i18n"
	"github.com/elionaz/panal/internal/mailbox"
	"github.com/elionaz/panal/internal/paths"
	"github.com/elionaz/panal/internal/profile"
	"github.com/elionaz/panal/internal/project"
	"github.com/elionaz/panal/internal/protocol"
	"github.com/elionaz/panal/internal/settings"
	"github.com/elionaz/panal/internal/transport"
)

// Output batching: one `pty:<id>` event per window or per chunk.
const (
	flushDelay = 8 * time.Millisecond
	flushBytes = 32 * 1024
	callTO     = 10 * time.Second
	gitTO      = 70 * time.Second  // diffs, worktree creation, merges
	pushTO     = 130 * time.Second // worktree.pr (the daemon allows 120s)
)

// Event names emitted to the frontend. Per-session events are suffixed
// with ":<sessionId>".
const (
	evPty        = "pty"             // base64 output chunk
	evReplayDone = "pty-replay-done" // scrollback replay finished
	evExit       = "pty-exit"        // exit code (number)
	evError      = "pty-error"       // stream ended unexpectedly (message)
	evDaemon     = "daemon:event"    // protocol.Event
	evResync     = "daemon:resync"   // (re)subscribed: reload lists
	evDown       = "daemon:down"     // event stream lost (message)

	evOpenSettings = "app:open-settings" // menu "Preferences…" (⌘,)
)

// App is bound to the frontend. It is a thin proxy over the daemon client:
// the daemon owns the PTYs, so quitting the app leaves sessions running.
type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	paths  paths.Paths
	client *client.Client
	emit   func(event string, data ...any) // runtime.EventsEmit; swapped in tests
	desk   desktop                         // OS notifications / Dock; nopDesktop in tests
	att    attention
	upd    updaterCtl // in-app updates (updates.go)

	wailsCtx context.Context // the Wails runtime context (nil in tests)

	startMu sync.Mutex                      // serialises daemon auto-start
	launch  func(ctx context.Context) error // starts the daemon; nil: this executable (tests swap it)

	mu      sync.Mutex
	streams map[string]*attachment

	// Notification-click focus (urlopen.go).
	focusMu       sync.Mutex
	started       bool   // startup ran: focusSession may be used
	frontendReady bool   // the frontend called TakePendingFocus
	pendingFocus  string // session to select once the frontend is ready

	grabs dropGrabs   // dropped screenshot thumbnails (drops.go)
	view  browserView // the Browser tab (browserview.go)
}

type attachment struct {
	stream *client.Stream
	out    *coalescer
	mu     sync.Mutex
	closed bool // set on intentional Detach: suppress error events
}

func NewApp() *App {
	return &App{
		streams: make(map[string]*attachment),
		desk:    nopDesktop{},
		att:     attention{unread: make(map[string]bool)},
		upd:     updaterCtl{reason: updStarting},
	}
}

func (a *App) startup(ctx context.Context) {
	p, err := paths.Default()
	if err != nil {
		log.Printf("panal: resolve paths: %v", err)
	}
	a.emit = func(event string, data ...any) { runtime.EventsEmit(ctx, event, data...) }
	a.wailsCtx = ctx
	a.desk = newDesktop(ctx, a.focusSession)
	lang := uiLang(p)
	installPreferencesMenu(i18n.T(lang, i18n.MenuPreferences), func() { a.emit(evOpenSettings) })
	// Off the startup callback: Sparkle is set up on the main thread, which
	// may be waiting for startup to return.
	go a.startUpdater(p)
	a.start(ctx, p)
	// Before the frontend hears of a drop: screenshot thumbnails vanish (drops.go).
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { a.grabDrops(paths) })
	a.focusMu.Lock()
	a.started = true
	a.focusMu.Unlock()
}

func (a *App) start(ctx context.Context, p paths.Paths) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.paths = p
	a.client = client.New(p.Socket())
	go a.eventLoop()
}

// shutdown detaches everything. It deliberately does not stop the daemon:
// sessions must survive the window.
func (a *App) shutdown(context.Context) {
	a.cancel()
	a.mu.Lock()
	streams := a.streams
	a.streams = make(map[string]*attachment)
	a.mu.Unlock()
	for _, at := range streams {
		at.close()
	}
}

// noSelfLaunch is set by TestMain: a test App must set launch.
var noSelfLaunch bool

// ensureDaemon starts the daemon from this very executable if it is not
// reachable, so the .app does not need a separately installed CLI.
func (a *App) ensureDaemon(ctx context.Context) error {
	a.startMu.Lock()
	defer a.startMu.Unlock()
	return a.ensureDaemonLocked(ctx)
}

func (a *App) ensureDaemonLocked(ctx context.Context) error {
	if transport.Alive(transport.Default(), a.paths.Socket()) {
		return nil
	}
	if a.launch != nil {
		return a.launch(ctx)
	}
	if noSelfLaunch {
		// Under `go test` this executable is app.test: `app.test daemon`
		// would rerun the whole suite, which launches another one… (a fork
		// bomb that took the machine down once).
		return errors.New("test App without launch tried to start a daemon")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	prepareDaemonEnv()
	return client.StartDaemon(ctx, daemonExe(exe), a.paths)
}

// call runs fn with a timeout, starting the daemon first if needed.
func (a *App) call(fn func(ctx context.Context) error) error {
	return a.callTimeout(callTO, fn)
}

func (a *App) callTimeout(to time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(a.ctx, to)
	defer cancel()
	if err := a.ensureDaemon(ctx); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	return fn(ctx)
}

// eventLoop forwards daemon events, resubscribing (and restarting the daemon
// if necessary) whenever the stream drops.
func (a *App) eventLoop() {
	backoff := 250 * time.Millisecond
	for a.ctx.Err() == nil {
		var ev *client.Events
		err := a.call(func(ctx context.Context) error {
			var err error
			// As a UI client: the daemon leaves notifications to us.
			ev, err = a.client.SubscribeAs(ctx, protocol.ClientUI)
			return err
		})
		if err != nil {
			a.emit(evDown, err.Error())
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		}
		backoff = 250 * time.Millisecond
		stop := context.AfterFunc(a.ctx, func() { ev.Close() })
		if list, err := a.ListSessions(); err == nil {
			a.syncAttention(list)
		}
		a.emit(evResync)
		for {
			e, err := ev.Next()
			if err != nil {
				break
			}
			a.onDaemonEvent(e)
			a.emit(evDaemon, e)
		}
		stop()
		ev.Close()
	}
}

// --- Projects & profiles ----------------------------------------------------

func (a *App) ListProjects() ([]project.Project, error) {
	var out []project.Project
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ProjectList(ctx)
		return err
	})
	return nonNil(out), err
}

// AddProject opens the native folder picker. It returns nil if cancelled.
func (a *App) AddProject() (*project.Project, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                i18n.T(uiLang(a.paths), i18n.DialogAddProject),
		CanCreateDirectories: true,
	})
	if err != nil || dir == "" {
		return nil, err
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return nil, err
	}
	var p project.Project
	err = a.call(func(ctx context.Context) (err error) {
		p, err = a.client.ProjectAdd(ctx, dir, "")
		return err
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProject renames a project or changes its badge (fields left out
// stay; an empty color or icon goes back to automatic).
func (a *App) UpdateProject(p protocol.ProjectUpdateParams) (*project.Project, error) {
	var out project.Project
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.UpdateProject(ctx, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ReorderProjects sets the sidebar's project order (ids first, in order).
func (a *App) ReorderProjects(ids []string) ([]project.Project, error) {
	var out []project.Project
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ReorderProjects(ctx, ids)
		return err
	})
	return nonNil(out), err
}

// ReorderSessions sets the order of a project's sessions (ids first).
func (a *App) ReorderSessions(projectID string, ids []string) ([]protocol.SessionInfo, error) {
	var out []protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ReorderSessions(ctx, projectID, ids)
		return err
	})
	return nonNil(out), err
}

func (a *App) RemoveProject(id string) error {
	return a.call(func(ctx context.Context) error { return a.client.ProjectRemove(ctx, id) })
}

func (a *App) ListProfiles() ([]profile.Profile, error) {
	var out []profile.Profile
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Profiles(ctx)
		return err
	})
	return nonNil(out), err
}

// ProfileCatalog lists the built-in agents with detection.
func (a *App) ProfileCatalog() ([]protocol.CatalogInfo, error) {
	var out []protocol.CatalogInfo
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ProfileCatalog(ctx)
		return err
	})
	return nonNil(out), err
}

// SaveProfile upserts a profiles.yaml entry; it returns every profile.
func (a *App) SaveProfile(e profile.Entry) ([]profile.Profile, error) {
	return a.profiles(func(ctx context.Context) ([]profile.Profile, error) { return a.client.SaveProfile(ctx, e) })
}

// DeleteProfile removes a custom profile or resets a built-in one.
func (a *App) DeleteProfile(id string) ([]profile.Profile, error) {
	return a.profiles(func(ctx context.Context) ([]profile.Profile, error) { return a.client.DeleteProfile(ctx, id) })
}

// ReorderProfiles moves ids to the front (the first one is ⌘N's).
func (a *App) ReorderProfiles(ids []string) ([]profile.Profile, error) {
	return a.profiles(func(ctx context.Context) ([]profile.Profile, error) { return a.client.ReorderProfiles(ctx, ids) })
}

// RescanProfiles re-detects installed agents.
func (a *App) RescanProfiles() ([]profile.Profile, error) {
	return a.profiles(a.client.RescanProfiles)
}

func (a *App) profiles(fn func(ctx context.Context) ([]profile.Profile, error)) ([]profile.Profile, error) {
	var out []profile.Profile
	err := a.call(func(ctx context.Context) (err error) {
		out, err = fn(ctx)
		return err
	})
	return nonNil(out), err
}

func (a *App) GetSettings() (settings.Settings, error) {
	var out settings.Settings
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Settings(ctx)
		return err
	})
	if err == nil {
		a.applyLanguage(out.UI)
	}
	return out, err
}

// SettingsPatch is what a Settings tab saves: only the sections it sets
// are sent, so the daemon keeps the others (a full settings.Settings would
// overwrite them with zero values).
type SettingsPatch struct {
	Notify   *settings.Notify   `json:"notify,omitempty"`
	Activity *settings.Activity `json:"activity,omitempty"`
	Git      *settings.Git      `json:"git,omitempty"`
	UI       *settings.UI       `json:"ui,omitempty"`
	Sessions *settings.Sessions `json:"sessions,omitempty"`
	Terminal *settings.Terminal `json:"terminal,omitempty"`
	Messages *settings.Messages `json:"messages,omitempty"`
	Browser  *settings.Browser  `json:"browser,omitempty"`
	MCP      *settings.MCP      `json:"mcp,omitempty"`
	History  *settings.History  `json:"history,omitempty"`
}

func (a *App) SaveSettings(s SettingsPatch) (settings.Settings, error) {
	var out settings.Settings
	err := a.call(func(ctx context.Context) error {
		return a.client.Call(ctx, protocol.MethodSettingsSave, s, &out)
	})
	if err == nil {
		a.applyLanguage(out.UI)
	}
	return out, err
}

// Instructions lists the instruction topics and, with projectID, what a
// session of that project receives.
func (a *App) Instructions(projectID string) (protocol.Instructions, error) {
	var out protocol.Instructions
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Instructions(ctx, projectID)
		return err
	})
	return out, err
}

// SaveInstruction saves a topic (new when oldSlug is "").
func (a *App) SaveInstruction(t protocol.InstructionTopic, oldSlug, projectID string) (protocol.Instructions, error) {
	var out protocol.Instructions
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.SaveInstruction(ctx, t, oldSlug, projectID)
		return err
	})
	return out, err
}

// DeleteInstruction removes a topic.
func (a *App) DeleteInstruction(slug, projectID string) (protocol.Instructions, error) {
	var out protocol.Instructions
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.DeleteInstruction(ctx, slug, projectID)
		return err
	})
	return out, err
}

// ReorderInstructions puts the topics in slugs' order.
func (a *App) ReorderInstructions(slugs []string, projectID string) (protocol.Instructions, error) {
	var out protocol.Instructions
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ReorderInstructions(ctx, slugs, projectID)
		return err
	})
	return out, err
}

// HandoffNote drafts the summary for continuing sessionID's work with
// another agent.
func (a *App) HandoffNote(sessionID string) (protocol.HandoffNote, error) {
	var out protocol.HandoffNote
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.HandoffNote(ctx, sessionID)
		return err
	})
	return out, err
}

// Handoff starts profileID with note where sessionID worked (where: same
// or worktree), or hands note to the live session targetID, and closes
// sessionID unless keep.
func (a *App) Handoff(sessionID, profileID, targetID, where, note string, keep bool, cols, rows int) (protocol.SessionInfo, error) {
	var out protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Handoff(ctx, protocol.HandoffParams{SessionID: sessionID, ProfileID: profileID, TargetID: targetID, Where: where,
			Note: note, Keep: keep, Cols: uint16(max(cols, 0)), Rows: uint16(max(rows, 0))})
		return err
	})
	return out, err
}

// BrowserStatus says whether sessions get their own browser and what was
// found for it (Chrome, npx).
func (a *App) BrowserStatus() (protocol.BrowserStatus, error) {
	var out protocol.BrowserStatus
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.BrowserStatus(ctx)
		return err
	})
	return out, err
}

// CloseBrowser stops sessionID's own browser.
func (a *App) CloseBrowser(sessionID string) error {
	return a.call(func(ctx context.Context) error {
		_, err := a.client.Browser(ctx, protocol.MethodBrowserClose, sessionID)
		return err
	})
}

// SystemLanguage is the language ui.language "auto" resolves to (macOS
// preferred languages, then LANG): "en" or "es".
func (a *App) SystemLanguage() string {
	return string(i18n.System.Detect())
}

// applyLanguage retitles the native menu item after a language change
// (notifications re-read settings.yaml by themselves).
func (a *App) applyLanguage(ui settings.UI) {
	if a.wailsCtx != nil {
		setPreferencesTitle(i18n.T(ui.Lang(), i18n.MenuPreferences))
		if r := a.updaterReason(); r != updStarting {
			setUpdatesMenu(updatesMenuText(ui.Lang(), r))
		}
	}
}

// uiLang is settings.yaml's ui.language in p, resolved.
func uiLang(p paths.Paths) i18n.Lang {
	if p.ConfigDir == "" {
		return i18n.Resolve(i18n.Auto)
	}
	st, _ := settings.Load(p.Settings())
	return st.UI.Lang()
}

// --- Sessions ---------------------------------------------------------------

func (a *App) ListSessions() ([]protocol.SessionInfo, error) {
	var out []protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Sessions(ctx)
		return err
	})
	return nonNil(out), err
}

func (a *App) Spawn(projectID, profileID string, cols, rows int) (protocol.SessionInfo, error) {
	return a.SpawnIn(projectID, profileID, cols, rows, SpawnOptions{})
}

// InstallAgent installs catalog agent profileID in a new shell session of
// project projectID (the command runs in view; a successful install
// rescans the agents).
func (a *App) InstallAgent(profileID, projectID string, cols, rows int) (protocol.SessionInfo, error) {
	var info protocol.SessionInfo
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		info, err = a.client.ProfileInstall(ctx, protocol.ProfileInstallParams{
			ProfileID: profileID, ProjectID: projectID, Cols: clampDim(cols), Rows: clampDim(rows)})
		return err
	})
	return info, err
}

// SpawnOptions choose where a session runs (see protocol.SpawnParams).
type SpawnOptions struct {
	Mode       string `json:"mode"` // same | worktree
	Branch     string `json:"branch"`
	Base       string `json:"base"`
	WorktreeID string `json:"worktreeId"` // reopen an existing worktree
	// TrustSetup / SkipSetup answer a setup_untrusted refusal.
	TrustSetup bool `json:"trustSetup"`
	SkipSetup  bool `json:"skipSetup"`
}

// SpawnIn launches a session in the project folder, a new worktree or an
// existing one.
func (a *App) SpawnIn(projectID, profileID string, cols, rows int, o SpawnOptions) (protocol.SessionInfo, error) {
	var info protocol.SessionInfo
	// Creating a worktree checks out files: allow more than a plain call.
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		info, err = a.client.Spawn(ctx, protocol.SpawnParams{
			ProjectID: projectID, ProfileID: profileID, Cols: clampDim(cols), Rows: clampDim(rows),
			Mode: o.Mode, Branch: o.Branch, Base: o.Base, WorktreeID: o.WorktreeID,
			TrustSetup: o.TrustSetup, SkipSetup: o.SkipSetup,
		})
		return err
	})
	return info, err
}

// Tail returns the last lines of a session's output as plain text (for
// the grid overview cards).
func (a *App) Tail(id string, lines int) ([]string, error) {
	var out []string
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.Tail(ctx, id, lines)
		return err
	})
	return nonNil(out), err
}

// Kill terminates a running session, or forgets an exited one.
func (a *App) Kill(id string) (protocol.SessionInfo, error) {
	var info protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		info, err = a.client.Kill(ctx, id)
		return err
	})
	return info, err
}

// Rename sets a session's display name (empty: back to the profile's).
func (a *App) Rename(id, name string) (protocol.SessionInfo, error) {
	var info protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		info, err = a.client.Rename(ctx, id, name)
		return err
	})
	return info, err
}

// Resume respawns a session suspended by a daemon restart (same id).
func (a *App) Resume(id string) (protocol.SessionInfo, error) {
	b, err := a.suspended(id, false, a.client.Resume)
	if err != nil || len(b.Sessions) == 0 {
		return protocol.SessionInfo{}, err
	}
	return b.Sessions[0], nil
}

// Discard forgets a suspended session and its saved scrollback.
func (a *App) Discard(id string) (protocol.SessionInfo, error) {
	b, err := a.suspended(id, false, a.client.Discard)
	if err != nil || len(b.Sessions) == 0 {
		return protocol.SessionInfo{}, err
	}
	return b.Sessions[0], nil
}

// ResumeAll resumes every suspended session (failures in Failed).
func (a *App) ResumeAll() (protocol.SessionBatch, error) {
	return a.suspended("", true, a.client.Resume)
}

// DiscardAll discards every suspended session.
func (a *App) DiscardAll() (protocol.SessionBatch, error) {
	return a.suspended("", true, a.client.Discard)
}

func (a *App) suspended(id string, all bool, fn func(context.Context, string, bool) (protocol.SessionBatch, error)) (protocol.SessionBatch, error) {
	var out protocol.SessionBatch
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = fn(ctx, id, all)
		return err
	})
	out.Sessions = nonNil(out.Sessions)
	return out, err
}

// Attach (re)opens the output stream of a session. The frontend must listen
// to `pty:<id>` before calling it: the daemon replays the scrollback first,
// then `pty-replay-done:<id>`, then live output. Attaching an already
// attached session replaces the stream (and replays again), which is what a
// reloaded frontend needs.
func (a *App) Attach(id string, cols, rows int) (protocol.SessionInfo, error) {
	a.Detach(id)
	var st *client.Stream
	err := a.call(func(ctx context.Context) (err error) {
		st, err = a.client.Attach(ctx, id, clampDim(cols), clampDim(rows))
		return err
	})
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	ptyEvent := evPty + ":" + id
	at := &attachment{stream: st}
	at.out = newCoalescer(flushDelay, flushBytes, func(b []byte) {
		a.emit(ptyEvent, base64.StdEncoding.EncodeToString(b))
	})
	a.mu.Lock()
	if old := a.streams[id]; old != nil { // lost a race with another Attach
		old.close()
	}
	a.streams[id] = at
	a.mu.Unlock()
	go a.pump(id, at)
	return st.Info, nil
}

// pump reads frames until the stream ends, translating them to events.
func (a *App) pump(id string, at *attachment) {
	defer a.forget(id, at)
	for {
		f, err := at.stream.Recv()
		if err != nil {
			at.out.Flush()
			if !at.isClosed() {
				a.emit(evError+":"+id, "disconnected from daemon: "+err.Error())
			}
			return
		}
		switch f.Type {
		case protocol.FrameOutput:
			at.out.Write(f.Payload)
		case protocol.FrameReplayDone:
			at.out.Flush()
			if !at.isClosed() {
				a.emit(evReplayDone + ":" + id)
			}
		case protocol.FrameExit:
			at.out.Flush()
			code, _ := protocol.ParseExit(f.Payload)
			if !at.isClosed() {
				a.emit(evExit+":"+id, code)
			}
			return
		case protocol.FrameError:
			at.out.Flush()
			if !at.isClosed() {
				a.emit(evError+":"+id, string(f.Payload))
			}
			return
		}
	}
}

// Detach closes the output stream; the session keeps running.
func (a *App) Detach(id string) {
	a.mu.Lock()
	at := a.streams[id]
	delete(a.streams, id)
	a.mu.Unlock()
	if at != nil {
		at.close()
	}
}

func (a *App) forget(id string, at *attachment) {
	at.out.Stop()
	at.stream.Close()
	a.mu.Lock()
	if a.streams[id] == at {
		delete(a.streams, id)
	}
	a.mu.Unlock()
}

// Input writes UTF-8 text (keystrokes, pastes) to the session.
func (a *App) Input(id, data string) error {
	return a.input(id, []byte(data))
}

// InputBase64 writes raw bytes (xterm's onBinary, e.g. legacy mouse reports).
func (a *App) InputBase64(id, b64 string) error {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	return a.input(id, b)
}

func (a *App) input(id string, b []byte) error {
	at := a.attached(id)
	if at == nil {
		return errors.New("session not attached")
	}
	return at.stream.Input(b)
}

func (a *App) Resize(id string, cols, rows int) error {
	c, r := clampDim(cols), clampDim(rows)
	if c == 0 || r == 0 {
		return nil
	}
	if at := a.attached(id); at != nil {
		return at.stream.Resize(c, r)
	}
	return a.call(func(ctx context.Context) error { return a.client.Resize(ctx, id, c, r) })
}

func (a *App) attached(id string) *attachment {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.streams[id]
}

func (at *attachment) close() {
	at.mu.Lock()
	at.closed = true
	at.mu.Unlock()
	at.out.Stop()
	at.stream.Close()
}

func (at *attachment) isClosed() bool {
	at.mu.Lock()
	defer at.mu.Unlock()
	return at.closed
}

func clampDim(n int) uint16 {
	if n <= 0 {
		return 0
	}
	return uint16(min(n, 1000))
}

// nonNil makes empty lists serialise as [] instead of null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// --- Git worktrees & diffs ----------------------------------------------------

// ProjectGit tells the spawn menu whether worktrees are available, the
// branches and the default base.
func (a *App) ProjectGit(projectID string) (protocol.ProjectGit, error) {
	var out protocol.ProjectGit
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.ProjectGit(ctx, projectID)
		return err
	})
	out.Branches = nonNil(out.Branches)
	return out, err
}

// SessionDiff returns a session's changes (with per-file patches).
func (a *App) SessionDiff(id string) (protocol.DiffResult, error) {
	var out protocol.DiffResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.SessionDiff(ctx, id, false)
		return err
	})
	return diffNonNil(out), err
}

// WorktreeDiff returns a worktree's changes (orphans included).
func (a *App) WorktreeDiff(id string) (protocol.DiffResult, error) {
	var out protocol.DiffResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreeDiff(ctx, id, false)
		return err
	})
	return diffNonNil(out), err
}

func diffNonNil(d protocol.DiffResult) protocol.DiffResult {
	d.Files = nonNil(d.Files)
	d.Commits = nonNil(d.Commits)
	return d
}

// ListWorktrees lists a project's panal worktrees.
func (a *App) ListWorktrees(projectID string) ([]protocol.WorktreeInfo, error) {
	var out []protocol.WorktreeInfo
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.Worktrees(ctx, projectID)
		return err
	})
	return nonNil(out), err
}

// RemoveWorktree removes a worktree; dryRun only reports what would be lost.
func (a *App) RemoveWorktree(id string, force, deleteBranch, dryRun bool) (protocol.WorktreeRemoveResult, error) {
	var out protocol.WorktreeRemoveResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: id, Force: force, DeleteBranch: deleteBranch, DryRun: dryRun})
		return err
	})
	return out, err
}

// SearchHistory searches the ended agent sessions (query "": the latest).
func (a *App) SearchHistory(query, projectID string, limit int) ([]history.Hit, error) {
	var out []history.Hit
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.HistorySearch(ctx, protocol.HistorySearchParams{Query: query, ProjectID: projectID, Limit: limit})
		return err
	})
	return nonNil(out), err
}

// HistoryDetail returns an archived session with its terminal text.
func (a *App) HistoryDetail(id string) (protocol.HistoryDetail, error) {
	var out protocol.HistoryDetail
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.HistoryGet(ctx, id)
		return err
	})
	return out, err
}

// DeleteHistory removes an archived session.
func (a *App) DeleteHistory(id string) error {
	return a.call(func(ctx context.Context) error { return a.client.HistoryDelete(ctx, id) })
}

// ResumeHistory starts a session that reopens an archived session's agent
// conversation.
func (a *App) ResumeHistory(id string, cols, rows int) (protocol.SessionInfo, error) {
	var out protocol.SessionInfo
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.HistoryResume(ctx, protocol.HistoryResumeParams{ID: id, Cols: uint16(max(cols, 0)), Rows: uint16(max(rows, 0))})
		return err
	})
	return out, err
}

// RemoveMergedWorktrees removes a project's orphaned worktrees whose branch
// is already merged; dryRun only lists them.
func (a *App) RemoveMergedWorktrees(projectID string, dryRun bool) (protocol.WorktreeRemoveMergedResult, error) {
	var out protocol.WorktreeRemoveMergedResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreeRemoveMerged(ctx, protocol.WorktreeRemoveMergedParams{ProjectID: projectID, DryRun: dryRun})
		return err
	})
	return out, err
}

// CopyIntoWorktree copies files from the main folder into a worktree;
// save also writes them into a new .panal.yaml.
func (a *App) CopyIntoWorktree(id string, files []string, save bool) (protocol.WorktreeCopyResult, error) {
	var out protocol.WorktreeCopyResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreeCopy(ctx, protocol.WorktreeCopyParams{ID: id, Files: files, Save: save})
		return err
	})
	return out, err
}

// MergeWorktree merges a worktree's branch into its base (main folder).
func (a *App) MergeWorktree(id string) (gitx.MergeResult, error) {
	var out gitx.MergeResult
	err := a.callTimeout(gitTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreeMerge(ctx, id)
		return err
	})
	return out, err
}

// PullRequestWorktree pushes a worktree's branch and returns the page that
// opens a pull request into its base; dryRun only reports remote and URL.
func (a *App) PullRequestWorktree(id string, dryRun bool) (gitx.PRResult, error) {
	var out gitx.PRResult
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.WorktreePR(ctx, id, dryRun)
		return err
	})
	return out, err
}

// ForgeStatus is the pull request of a session's (sessionID) or a
// worktree's (worktreeID) branch on GitHub / GitLab, or why it cannot be
// shown (ForgeStatus.Unavailable).
func (a *App) ForgeStatus(sessionID, worktreeID, branch string) (protocol.ForgeStatus, error) {
	var out protocol.ForgeStatus
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.ForgeStatus(ctx, protocol.ForgeRef{SessionID: sessionID, WorktreeID: worktreeID, Branch: branch})
		return err
	})
	return out, err
}

// ForgeComments lists the comments and reviews of pull request number.
func (a *App) ForgeComments(sessionID, worktreeID, branch string, number int) ([]forge.Comment, error) {
	var out []forge.Comment
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.ForgeComments(ctx, protocol.ForgeCommentsParams{
			ForgeRef: protocol.ForgeRef{SessionID: sessionID, WorktreeID: worktreeID, Branch: branch}, Number: number})
		return err
	})
	if out == nil {
		out = []forge.Comment{}
	}
	return out, err
}

// ForgeMerge merges pull request number on the host (method: squash, merge
// or rebase; head: the branch it was shown for; deleteBranch removes the
// branch there, never locally).
func (a *App) ForgeMerge(sessionID, worktreeID, branch string, number int, method, head string, deleteBranch bool) (protocol.ForgeStatus, error) {
	var out protocol.ForgeStatus
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.ForgeMerge(ctx, protocol.ForgeMergeParams{
			ForgeRef: protocol.ForgeRef{SessionID: sessionID, WorktreeID: worktreeID, Branch: branch}, Number: number, Method: method, DeleteBranch: deleteBranch, Head: head})
		return err
	})
	return out, err
}

// ForgeCreate pushes the branch and opens its pull request.
// ForgeList lists the pull requests of the branches a session's (or
// worktree's) folder worked on; refresh skips the daemon's cache.
func (a *App) ForgeList(sessionID, worktreeID string, refresh bool) (protocol.ForgeList, error) {
	var out protocol.ForgeList
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.ForgeList(ctx, protocol.ForgeListParams{
			ForgeRef: protocol.ForgeRef{SessionID: sessionID, WorktreeID: worktreeID}, Refresh: refresh})
		return err
	})
	if out.PRs == nil {
		out.PRs = []protocol.ForgeListItem{}
	}
	return out, err
}

func (a *App) ForgeCreate(sessionID, worktreeID, title, body string, draft bool) (protocol.ForgeStatus, error) {
	var out protocol.ForgeStatus
	err := a.callTimeout(pushTO, func(ctx context.Context) (err error) {
		out, err = a.client.ForgeCreate(ctx, protocol.ForgeCreateParams{
			ForgeRef: protocol.ForgeRef{SessionID: sessionID, WorktreeID: worktreeID}, Title: title, Body: body, Draft: draft})
		return err
	})
	return out, err
}

// OpenWorktree opens a worktree (id) or a session's folder (sessionID) in
// the editor or Finder (target: editor | finder).
func (a *App) OpenWorktree(id, sessionID, target string) error {
	return a.call(func(ctx context.Context) error {
		return a.client.WorktreeOpen(ctx, protocol.WorktreeOpenParams{ID: id, SessionID: sessionID, Target: target})
	})
}

// MessagePeers lists the sessions of sessionID's project, as agents see
// them (panal msg peers).
func (a *App) MessagePeers(sessionID string) (protocol.MsgPeers, error) {
	var out protocol.MsgPeers
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.MsgPeers(ctx, sessionID)
		return err
	})
	return out, err
}

// ListMessages lists the messages sent by or to sessionID (all of the
// project when sessionID is empty).
func (a *App) ListMessages(sessionID, projectID string) ([]mailbox.Message, error) {
	var out []mailbox.Message
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.MsgList(ctx, protocol.MsgListParams{SessionID: sessionID, ProjectID: projectID})
		return err
	})
	return out, err
}

// SendMessage sends a message from the user to a session (to: an id) or
// to every session of projectID (to: "all").
func (a *App) SendMessage(to, projectID, body string) ([]mailbox.Message, error) {
	var out []mailbox.Message
	err := a.call(func(ctx context.Context) (err error) {
		out, err = a.client.MsgSend(ctx, protocol.MsgSendParams{To: to, ProjectID: projectID, Body: body})
		return err
	})
	return out, err
}

// ReleaseMessages delivers every message held in a project (its messages
// are manual); returns for how many sessions there were.
func (a *App) ReleaseMessages(projectID string) (int, error) {
	var n int
	err := a.call(func(ctx context.Context) (err error) {
		n, err = a.client.MsgRelease(ctx, projectID)
		return err
	})
	return n, err
}

// DeliverMessages types a fixed line into sessionID's terminal telling the
// agent it has messages (never the messages themselves); returns how many
// were pending.
func (a *App) DeliverMessages(sessionID string) (int, error) {
	var n int
	err := a.call(func(ctx context.Context) (err error) {
		n, err = a.client.MsgDeliver(ctx, sessionID)
		return err
	})
	return n, err
}
