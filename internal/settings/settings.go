// Package settings reads the optional user settings file
// (~/.config/panal/settings.yaml). Every toggle defaults to "on".
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/elionaz/panal/internal/atomicfile"
	"github.com/elionaz/panal/internal/fileformat"
	"github.com/elionaz/panal/internal/i18n"
)

// Notify controls attention notifications.
type Notify struct {
	Enabled bool `json:"enabled"` // native notifications at all
	Sound   bool `json:"sound"`   // sound for permission/question notifications
	// Per kind: an approval (permission), a question, a finished turn, an
	// error, a turn that looks stalled (quiet).
	OnApproval bool `json:"notifyOnApproval"`
	OnQuestion bool `json:"notifyOnQuestion"`
	OnDone     bool `json:"notifyOnDone"`
	OnError    bool `json:"notifyOnError"`
	OnStall    bool `json:"notifyOnStall"`
	// Fallback is what to do when native notifications are unavailable
	// (no .app bundle) or not authorized: none (drop them) | osascript
	// (AppleScript, which macOS shows as Script Editor). omitempty: a client
	// that does not know the field keeps its saved value.
	Fallback string `json:"fallback,omitempty"`
}

// Notify.Fallback values.
const (
	FallbackNone      = "none"
	FallbackOsascript = "osascript"
)

func normFallback(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == FallbackOsascript {
		return FallbackOsascript
	}
	return FallbackNone
}

// UsesOsascript reports whether notifications may fall back to osascript.
func (n Notify) UsesOsascript() bool { return normFallback(n.Fallback) == FallbackOsascript }

// Activity controls activity tracking.
type Activity struct {
	// StallSeconds: a hook-driven turn with neither output nor hook events
	// for this long is flagged as stalled. 0 disables stall detection.
	StallSeconds int `json:"stallSeconds"`
}

// StallAfter is StallSeconds as a duration (0 = disabled).
func (a Activity) StallAfter() time.Duration {
	return time.Duration(max(a.StallSeconds, 0)) * time.Second
}

// DefaultStallSeconds is activity.stall_seconds when unset.
const DefaultStallSeconds = 180

// Git configures worktree sessions and the diff actions.
type Git struct {
	// Editor opens a worktree from "Open in editor": a command on PATH
	// ("code", "cursor -n"), an absolute path or a macOS app name
	// ("Visual Studio Code"). Empty: `code` if found, else Finder.
	Editor string `json:"editor"`
	// WorktreeDir overrides where worktrees are created (default
	// ~/.panal/worktrees; "~/" is expanded).
	WorktreeDir string `json:"worktreeDir"`
	// DefaultMode is what the spawn menu preselects for agents: worktree
	// (the default: each one on its own branch) or same. Shells always
	// start in the project folder.
	DefaultMode string `json:"defaultMode"`
}

// Spawn modes (Git.DefaultMode, session.spawn's mode).
const (
	ModeSame     = "same"
	ModeWorktree = "worktree"
)

// UI configures the desktop app (and the texts the daemon renders for it).
//
// Its JSON fields are omitempty: a client saving only one of them (the
// General tab the language, the Appearance tab the appearance) sends the
// section without the other, which then keeps its saved value.
type UI struct {
	// Language is auto | en | es. auto follows the system's preferred
	// languages (macOS AppleLanguages, then LC_ALL/LC_MESSAGES/LANG): the
	// first of en/es found, else English.
	Language string `json:"language,omitempty"`
	// Appearance is the app chrome's color scheme: system (follow macOS)
	// | dark | light.
	Appearance string `json:"appearance,omitempty"`
}

// UI.Appearance values.
const (
	AppearanceSystem = "system"
	AppearanceDark   = "dark"
	AppearanceLight  = "light"
)

func normAppearance(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case AppearanceDark, AppearanceLight:
		return v
	}
	return AppearanceSystem
}

// Terminal configures the desktop app's terminals.
type Terminal struct {
	// FontFamily is a CSS font-family list.
	FontFamily string `json:"fontFamily"`
	// FontSize in points (MinFontSize..MaxFontSize).
	FontSize int `json:"fontSize"`
	// LineHeight is a multiple of the font size (1.0..2.0).
	LineHeight float64 `json:"lineHeight"`
	// CursorStyle is block | bar | underline.
	CursorStyle string `json:"cursorStyle"`
	// Theme names a built-in palette (Themes); unknown names fall back to
	// DefaultTheme.
	Theme string `json:"theme"`
}

// Terminal defaults and limits.
const (
	DefaultFontFamily = `"SF Mono", Menlo, Monaco, monospace`
	DefaultFontSize   = 13
	MinFontSize       = 8
	MaxFontSize       = 32
	DefaultLineHeight = 1.15
	DefaultCursor     = "block"
	DefaultTheme      = "bright-lights"
)

// Themes are the built-in terminal palettes, each named after an album (the
// app defines their colors): dark ones first, then light ones.
var Themes = []string{"bright-lights", "room-on-fire", "is-this-it", "funeral", "silent-alarm", "z",
	"it-still-moves", "alphabetical", "hot-fuss"}

// ThemeAliases maps retired theme names to the closest current palette, so an
// older settings.yaml keeps a similar look.
var ThemeAliases = map[string]string{
	"panal-dark":      "bright-lights",
	"panal-light":     "alphabetical",
	"solarized-dark":  "z",
	"solarized-light": "hot-fuss",
	"dracula":         "funeral",
	"gruvbox-dark":    "is-this-it",
	"nord":            "silent-alarm",
}

func normTerminal(t Terminal) Terminal {
	t.FontFamily = strings.TrimSpace(t.FontFamily)
	if t.FontFamily == "" {
		t.FontFamily = DefaultFontFamily
	}
	if t.FontSize == 0 {
		t.FontSize = DefaultFontSize
	}
	t.FontSize = min(max(t.FontSize, MinFontSize), MaxFontSize)
	if t.LineHeight == 0 {
		t.LineHeight = DefaultLineHeight
	}
	// Two decimals at most, within 1.0..2.0.
	t.LineHeight = min(max(math.Round(t.LineHeight*100)/100, 1), 2)
	switch t.CursorStyle = strings.ToLower(strings.TrimSpace(t.CursorStyle)); t.CursorStyle {
	case "block", "bar", "underline":
	default:
		t.CursorStyle = DefaultCursor
	}
	t.Theme = strings.ToLower(strings.TrimSpace(t.Theme))
	if alias, ok := ThemeAliases[t.Theme]; ok {
		t.Theme = alias
	}
	if !slices.Contains(Themes, t.Theme) {
		t.Theme = DefaultTheme
	}
	return t
}

// Lang resolves Language to the language to render in.
func (u UI) Lang() i18n.Lang { return i18n.Resolve(u.Language) }

// Sessions configures what happens to sessions across daemon restarts.
type Sessions struct {
	// Restore decides what the daemon does at startup with the sessions
	// that were live when the previous one stopped: ask (keep them
	// suspended until the user resumes or discards them), always (resume
	// them right away) or never (discard them).
	Restore string `json:"restore"`
}

// Sessions.Restore values.
const (
	RestoreAsk    = "ask"
	RestoreAlways = "always"
	RestoreNever  = "never"
)

func normRestore(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case RestoreAlways, RestoreNever:
		return v
	}
	return RestoreAsk
}

// Messages configures messages between sessions (panal msg).
type Messages struct {
	// Enabled: sessions may send each other messages, and agent hooks
	// deliver them.
	Enabled bool `json:"enabled"`
	// AutoDeliver (on by default): an idle agent is told about new messages
	// without the Deliver click: Claude, and agents that report the end of
	// their turn (see internal/daemon/autodeliver.go).
	AutoDeliver bool `json:"autoDeliver"`
}

// Browser configures the isolated Chrome panal gives each session for
// agents' browser work (see internal/browser).
type Browser struct {
	// Enabled (default): Claude Code and codex sessions get a browser MCP
	// server bound to the session's own Chrome; every session can start it
	// with panal browser.
	Enabled bool `json:"enabled"`
	// Headless (default): no window; the app shows it in the session's
	// Browser tab.
	Headless bool `json:"headless"`
	// Path of the Chrome-compatible executable; "" finds one.
	Path string `json:"path"`
}

// MCP configures panal's own MCP server for agents (panal mcp).
type MCP struct {
	// Enabled (default): agent sessions with an integration (Claude Code,
	// codex, Gemini CLI, opencode, Cursor CLI) get the "panal" MCP server:
	// tools to message and wait for the other sessions and read panal's
	// state.
	Enabled bool `json:"enabled"`
}

// History configures the archive of ended agent sessions (see
// internal/history).
type History struct {
	// Enabled (default): an agent session that ends is kept, searchable.
	Enabled bool `json:"enabled"`
	// Keep is how many sessions are kept (the oldest go first).
	Keep int `json:"keep"`
}

// DefaultHistoryKeep is history.keep when unset.
const DefaultHistoryKeep = 300

// Settings is the parsed file.
type Settings struct {
	Notify   Notify   `json:"notify"`
	Activity Activity `json:"activity"`
	Git      Git      `json:"git"`
	UI       UI       `json:"ui"`
	Sessions Sessions `json:"sessions"`
	Terminal Terminal `json:"terminal"`
	Messages Messages `json:"messages"`
	Browser  Browser  `json:"browser"`
	MCP      MCP      `json:"mcp"`
	History  History  `json:"history"`
}

type fileHistory struct {
	Enabled *bool `yaml:"enabled"`
	Keep    *int  `yaml:"keep"`
}

type fileMCP struct {
	Enabled *bool `yaml:"enabled"`
}

type fileBrowser struct {
	Enabled  *bool   `yaml:"enabled"`
	Headless *bool   `yaml:"headless"`
	Path     *string `yaml:"path"`
}

type fileMessages struct {
	Enabled     *bool `yaml:"enabled"`
	AutoDeliver *bool `yaml:"auto_deliver"`
}

type fileNotify struct {
	Enabled    *bool   `yaml:"enabled"`
	Sound      *bool   `yaml:"sound"`
	OnApproval *bool   `yaml:"notify_on_approval"`
	OnQuestion *bool   `yaml:"notify_on_question"`
	OnDone     *bool   `yaml:"notify_on_done"`
	OnError    *bool   `yaml:"notify_on_error"`
	OnStall    *bool   `yaml:"notify_on_stall"`
	Fallback   *string `yaml:"fallback"`
}

type fileActivity struct {
	StallSeconds *int `yaml:"stall_seconds"`
}

type fileGit struct {
	Editor      *string `yaml:"editor"`
	WorktreeDir *string `yaml:"worktree_dir"`
	DefaultMode *string `yaml:"default_mode"`
}

type fileUI struct {
	Language   *string `yaml:"language"`
	Appearance *string `yaml:"appearance"`
}

type fileSessions struct {
	Restore *string `yaml:"restore"`
}

type fileTerminal struct {
	FontFamily  *string  `yaml:"font_family"`
	FontSize    *int     `yaml:"font_size"`
	LineHeight  *float64 `yaml:"line_height"`
	CursorStyle *string  `yaml:"cursor_style"`
	Theme       *string  `yaml:"theme"`
}

// Format is the settings.yaml format version (see internal/fileformat).
// Files from before 0.4.0 have no version key: they are format 1.
const Format = 1

// file is the decoded document; `version` is read from the node first
// (fileformat.YAMLVersion), so it is not a field here.
type file struct {
	Notify   fileNotify   `yaml:"notify"`
	Activity fileActivity `yaml:"activity"`
	Git      fileGit      `yaml:"git"`
	UI       fileUI       `yaml:"ui"`
	Sessions fileSessions `yaml:"sessions"`
	Terminal fileTerminal `yaml:"terminal"`
	Messages fileMessages `yaml:"messages"`
	Browser  fileBrowser  `yaml:"browser"`
	MCP      fileMCP      `yaml:"mcp"`
	History  fileHistory  `yaml:"history"`
}

const header = "# panal settings (see README)\n"

// Save writes s to path atomically (keeping a .bak of the previous file).
// The existing file is edited in place: unknown keys and comments are kept,
// only the known keys are set. A file that does not parse is replaced; one
// written by a newer panal is left alone (fileformat.TooNewError).
func Save(path string, s Settings) error {
	var doc yaml.Node
	if old, err := os.ReadFile(path); err == nil {
		if yaml.Unmarshal(old, &doc) != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			doc = yaml.Node{}
		} else if _, err := fileformat.YAMLVersion("settings.yaml", &doc, Format); fileformat.Protected(err) {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, HeadComment: header[2 : len(header)-1],
			Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	setVersion(root)
	b := strconv.FormatBool
	setKey(root, []string{"notify", "enabled"}, b(s.Notify.Enabled), "!!bool")
	setKey(root, []string{"notify", "sound"}, b(s.Notify.Sound), "!!bool")
	setKey(root, []string{"notify", "notify_on_approval"}, b(s.Notify.OnApproval), "!!bool")
	setKey(root, []string{"notify", "notify_on_question"}, b(s.Notify.OnQuestion), "!!bool")
	setKey(root, []string{"notify", "notify_on_done"}, b(s.Notify.OnDone), "!!bool")
	setKey(root, []string{"notify", "notify_on_error"}, b(s.Notify.OnError), "!!bool")
	setKey(root, []string{"notify", "notify_on_stall"}, b(s.Notify.OnStall), "!!bool")
	setKey(root, []string{"notify", "fallback"}, normFallback(s.Notify.Fallback), "!!str")
	setKey(root, []string{"activity", "stall_seconds"}, strconv.Itoa(max(s.Activity.StallSeconds, 0)), "!!int")
	setKey(root, []string{"git", "editor"}, strings.TrimSpace(s.Git.Editor), "!!str")
	setKey(root, []string{"git", "worktree_dir"}, strings.TrimSpace(s.Git.WorktreeDir), "!!str")
	setKey(root, []string{"git", "default_mode"}, normMode(s.Git.DefaultMode), "!!str")
	setKey(root, []string{"ui", "language"}, i18n.NormalizeSetting(s.UI.Language), "!!str")
	setKey(root, []string{"ui", "appearance"}, normAppearance(s.UI.Appearance), "!!str")
	setKey(root, []string{"sessions", "restore"}, normRestore(s.Sessions.Restore), "!!str")
	setKey(root, []string{"messages", "enabled"}, b(s.Messages.Enabled), "!!bool")
	setKey(root, []string{"messages", "auto_deliver"}, b(s.Messages.AutoDeliver), "!!bool")
	setKey(root, []string{"browser", "enabled"}, b(s.Browser.Enabled), "!!bool")
	setKey(root, []string{"browser", "headless"}, b(s.Browser.Headless), "!!bool")
	setKey(root, []string{"browser", "path"}, strings.TrimSpace(s.Browser.Path), "!!str")
	setKey(root, []string{"mcp", "enabled"}, b(s.MCP.Enabled), "!!bool")
	setKey(root, []string{"history", "enabled"}, b(s.History.Enabled), "!!bool")
	setKey(root, []string{"history", "keep"}, strconv.Itoa(normKeep(s.History.Keep)), "!!int")
	term := normTerminal(s.Terminal)
	setKey(root, []string{"terminal", "font_family"}, term.FontFamily, "!!str")
	setKey(root, []string{"terminal", "font_size"}, strconv.Itoa(term.FontSize), "!!int")
	setKey(root, []string{"terminal", "line_height"}, strconv.FormatFloat(term.LineHeight, 'f', -1, 64), "!!float")
	setKey(root, []string{"terminal", "cursor_style"}, term.CursorStyle, "!!str")
	setKey(root, []string{"terminal", "theme"}, term.Theme, "!!str")
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return atomicfile.Write(path, buf.Bytes(), 0o600)
}

// setVersion sets the top-level version key, adding it first in the file
// (a file's leading comment, separated by a blank line, stays above it).
func setVersion(m *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "version" {
			setKey(m, []string{"version"}, strconv.Itoa(Format), "!!int")
			return
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"}
	v := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(Format)}
	m.Content = append([]*yaml.Node{k, v}, m.Content...)
}

// setKey sets the scalar at path inside mapping m, creating (or replacing
// non-mapping) intermediate nodes as needed.
func setKey(m *yaml.Node, path []string, value, tag string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Value != path[0] {
			continue
		}
		if len(path) == 1 {
			v.Kind, v.Tag, v.Value, v.Style, v.Content, v.Alias = yaml.ScalarNode, tag, value, 0, nil, nil
			return
		}
		if v.Kind != yaml.MappingNode {
			*v = yaml.Node{Kind: yaml.MappingNode, LineComment: v.LineComment, HeadComment: v.HeadComment}
		}
		setKey(v, path[1:], value, tag)
		return
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: path[0]}
	if len(path) == 1 {
		m.Content = append(m.Content, k, &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value})
		return
	}
	v := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content, k, v)
	setKey(v, path[1:], value, tag)
}

// Defaults returns the settings used when the file (or a key) is missing.
func Defaults() Settings {
	return Settings{
		Notify:   Notify{Enabled: true, Sound: true, OnApproval: true, OnQuestion: true, OnDone: true, OnError: true, OnStall: true, Fallback: FallbackNone},
		Activity: Activity{StallSeconds: DefaultStallSeconds},
		Git:      Git{DefaultMode: ModeWorktree},
		UI:       UI{Language: i18n.Auto, Appearance: AppearanceSystem},
		Sessions: Sessions{Restore: RestoreAsk},
		Terminal: normTerminal(Terminal{}),
		Messages: Messages{Enabled: true, AutoDeliver: true},
		Browser:  Browser{Enabled: true, Headless: true},
		MCP:      MCP{Enabled: true},
		History:  History{Enabled: true, Keep: DefaultHistoryKeep},
	}
}

// normMode: same or worktree; anything else is the default (worktree:
// each new agent gets its own branch and folder).
func normMode(m string) string {
	if strings.TrimSpace(m) == ModeSame {
		return ModeSame
	}
	return ModeWorktree
}

// Parse decodes YAML, filling missing keys with defaults. A file written by
// a newer panal is still read as far as this build understands it, and
// returned with a fileformat.TooNewError.
func Parse(data []byte) (Settings, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Defaults(), fmt.Errorf("parse settings: %w", err)
	}
	// The version first: a newer file whose keys changed shape must still
	// be reported as newer, not as broken.
	_, vErr := fileformat.YAMLVersion("settings.yaml", &doc, Format)
	var f file
	if doc.Kind != 0 {
		if err := doc.Decode(&f); err != nil {
			if vErr != nil {
				return Defaults(), vErr
			}
			return Defaults(), fmt.Errorf("parse settings: %w", err)
		}
	}
	s := Defaults()
	set := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	set(&s.Notify.Enabled, f.Notify.Enabled)
	set(&s.Notify.Sound, f.Notify.Sound)
	set(&s.Notify.OnApproval, f.Notify.OnApproval)
	set(&s.Notify.OnQuestion, f.Notify.OnQuestion)
	set(&s.Notify.OnDone, f.Notify.OnDone)
	set(&s.Notify.OnError, f.Notify.OnError)
	set(&s.Notify.OnStall, f.Notify.OnStall)
	set(&s.Messages.Enabled, f.Messages.Enabled)
	set(&s.Messages.AutoDeliver, f.Messages.AutoDeliver)
	set(&s.Browser.Enabled, f.Browser.Enabled)
	set(&s.MCP.Enabled, f.MCP.Enabled)
	set(&s.History.Enabled, f.History.Enabled)
	if f.History.Keep != nil {
		s.History.Keep = normKeep(*f.History.Keep)
	}
	set(&s.Browser.Headless, f.Browser.Headless)
	if f.Browser.Path != nil {
		s.Browser.Path = strings.TrimSpace(*f.Browser.Path)
	}
	if f.Notify.Fallback != nil {
		s.Notify.Fallback = normFallback(*f.Notify.Fallback)
	}
	if f.Activity.StallSeconds != nil {
		s.Activity.StallSeconds = max(*f.Activity.StallSeconds, 0)
	}
	if f.Git.Editor != nil {
		s.Git.Editor = strings.TrimSpace(*f.Git.Editor)
	}
	if f.Git.WorktreeDir != nil {
		s.Git.WorktreeDir = strings.TrimSpace(*f.Git.WorktreeDir)
	}
	if f.Git.DefaultMode != nil {
		s.Git.DefaultMode = normMode(*f.Git.DefaultMode)
	}
	if f.UI.Language != nil {
		s.UI.Language = i18n.NormalizeSetting(*f.UI.Language)
	}
	if f.UI.Appearance != nil {
		s.UI.Appearance = normAppearance(*f.UI.Appearance)
	}
	if f.Sessions.Restore != nil {
		s.Sessions.Restore = normRestore(*f.Sessions.Restore)
	}
	ft := f.Terminal
	if ft.FontFamily != nil {
		s.Terminal.FontFamily = *ft.FontFamily
	}
	if ft.FontSize != nil {
		s.Terminal.FontSize = *ft.FontSize
	}
	if ft.LineHeight != nil {
		s.Terminal.LineHeight = *ft.LineHeight
	}
	if ft.CursorStyle != nil {
		s.Terminal.CursorStyle = *ft.CursorStyle
	}
	if ft.Theme != nil {
		s.Terminal.Theme = *ft.Theme
	}
	s.Terminal = normTerminal(s.Terminal)
	return s, vErr
}

// Load reads path. A missing file yields Defaults(); a broken one yields
// Defaults() plus the error, so callers can log it and carry on.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), err
	}
	return Parse(data)
}

// ShouldNotify applies the toggles to an attention kind.
func (n Notify) ShouldNotify(kind string) bool {
	if !n.Enabled {
		return false
	}
	switch kind {
	case "permission":
		return n.OnApproval
	case "question":
		return n.OnQuestion
	case "done":
		return n.OnDone
	case "error":
		return n.OnError
	case "stall":
		return n.OnStall
	}
	return true // a kind this version doesn't know
}

// WantsSound reports whether a notification of kind should play a sound:
// only requests that block the agent (permission, question) do.
func (n Notify) WantsSound(kind string) bool {
	return n.Sound && (kind == "permission" || kind == "question")
}

// normKeep bounds history.keep: at least 1, at most 5000 (0 or less is the
// default).
func normKeep(n int) int {
	if n <= 0 {
		return DefaultHistoryKeep
	}
	return min(n, 5000)
}
