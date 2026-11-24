package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultsWhenMissing(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if s != Defaults() || !s.Notify.Enabled || !s.Notify.Sound || !s.Notify.OnDone {
		t.Fatalf("got %+v", s)
	}
}

func TestPartialFileKeepsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.yaml")
	os.WriteFile(p, []byte("notify:\n  sound: false\n"), 0o600)
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Notify.Enabled || s.Notify.Sound || !s.Notify.OnDone {
		t.Fatalf("got %+v", s)
	}
	if s.Notify.WantsSound("permission") {
		t.Fatal("sound disabled")
	}
}

func TestBrokenFileFallsBack(t *testing.T) {
	s, err := Parse([]byte("notify: [nope"))
	if err == nil || s != Defaults() {
		t.Fatalf("got %+v, %v", s, err)
	}
}

func TestShouldNotify(t *testing.T) {
	d := Defaults().Notify
	for _, k := range []string{"permission", "question", "done", "error", "stall", "future-kind"} {
		if !d.ShouldNotify(k) {
			t.Errorf("defaults: %s off", k)
		}
	}
	// Each toggle turns off its kind only.
	for kind, off := range map[string]func(*Notify){
		"permission": func(n *Notify) { n.OnApproval = false },
		"question":   func(n *Notify) { n.OnQuestion = false },
		"done":       func(n *Notify) { n.OnDone = false },
		"error":      func(n *Notify) { n.OnError = false },
		"stall":      func(n *Notify) { n.OnStall = false },
	} {
		n := d
		off(&n)
		for _, k := range []string{"permission", "question", "done", "error", "stall"} {
			if got := n.ShouldNotify(k); got != (k != kind) {
				t.Errorf("%s off: ShouldNotify(%s) = %v", kind, k, got)
			}
		}
	}
	if !d.WantsSound("question") || d.WantsSound("done") {
		t.Fatal("sound kinds")
	}
	if (Notify{}).ShouldNotify("permission") {
		t.Fatal("disabled")
	}
}

func TestNotifyFallback(t *testing.T) {
	if d := Defaults().Notify; d.Fallback != FallbackNone || d.UsesOsascript() {
		t.Fatalf("default fallback = %q", d.Fallback)
	}
	for in, want := range map[string]string{"osascript": FallbackOsascript, " OSAScript ": FallbackOsascript, "none": FallbackNone, "bogus": FallbackNone, "": FallbackNone} {
		s, err := Parse([]byte("notify:\n  fallback: \"" + in + "\"\n"))
		if err != nil || s.Notify.Fallback != want {
			t.Errorf("fallback %q: got %q %v, want %q", in, s.Notify.Fallback, err, want)
		}
	}
	if !(Notify{Fallback: FallbackOsascript}).UsesOsascript() || (Notify{}).UsesOsascript() {
		t.Fatal("UsesOsascript")
	}
	// Saved in place next to the other notify keys; an empty value is none.
	p := filepath.Join(t.TempDir(), "settings.yaml")
	os.WriteFile(p, []byte("notify:\n  sound: false # quiet\n"), 0o600)
	st := Defaults()
	st.Notify.Sound = false
	st.Notify.Fallback = FallbackOsascript
	if err := Save(p, st); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "fallback: osascript") || !strings.Contains(string(data), "# quiet") {
		t.Fatalf("file:\n%s", data)
	}
	if got, err := Load(p); err != nil || got != st {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	st.Notify.Fallback = ""
	if err := Save(p, st); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(p); got.Notify.Fallback != FallbackNone {
		t.Fatalf("saved %q", got.Notify.Fallback)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.yaml")
	os.WriteFile(p, []byte("notify:\n  sound: false\n"), 0o600)
	want := Settings{Notify: Notify{Enabled: true, Sound: true, OnApproval: true, OnQuestion: false, OnDone: false, OnError: true, Fallback: FallbackOsascript}, Activity: Activity{StallSeconds: 60},
		Git: Git{Editor: "cursor -n", WorktreeDir: "~/wt", DefaultMode: ModeWorktree}, UI: UI{Language: "es", Appearance: AppearanceLight},
		Sessions: Sessions{Restore: RestoreAlways},
		Terminal: Terminal{FontFamily: "Menlo", FontSize: 14, LineHeight: 1.2, CursorStyle: "bar", Theme: "silent-alarm"},
		History:  History{Enabled: false, Keep: 120}}
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || got != want {
		t.Fatalf("got %+v %v", got, err)
	}
	if bak, _ := os.ReadFile(p + ".bak"); string(bak) != "notify:\n  sound: false\n" {
		t.Fatalf(".bak = %q", bak)
	}
}

func TestStallDefaultsAndParse(t *testing.T) {
	d := Defaults()
	if d.Activity.StallSeconds != 180 || !d.Notify.OnStall || d.Activity.StallAfter() != 180*time.Second {
		t.Fatalf("defaults %+v", d)
	}
	s, err := Parse([]byte("activity:\n  stall_seconds: 0\nnotify:\n  notify_on_stall: false\n"))
	if err != nil || s.Activity.StallSeconds != 0 || s.Notify.OnStall || s.Activity.StallAfter() != 0 {
		t.Fatalf("got %+v %v", s, err)
	}
	if s.Notify.ShouldNotify("stall") || !d.Notify.ShouldNotify("stall") || d.Notify.WantsSound("stall") {
		t.Fatal("stall toggles")
	}
}

func TestSavePreservesUnknownKeysAndComments(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.yaml")
	orig := "# mis ajustes\nnotify:\n  sound: false # silencio\n  future_toggle: yes\ntheme: dark\nextra:\n  - a\n  - b\n"
	os.WriteFile(p, []byte(orig), 0o600)
	want := Defaults()
	want.Notify.Sound = true
	want.Notify.OnStall = false
	want.Activity.StallSeconds = 300
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || got != want {
		t.Fatalf("round trip: got %+v %v", got, err)
	}
	data, _ := os.ReadFile(p)
	text := string(data)
	for _, keep := range []string{"# mis ajustes", "future_toggle: yes", "theme: dark", "- a", "# silencio", "stall_seconds: 300", "notify_on_stall: false"} {
		if !strings.Contains(text, keep) {
			t.Errorf("lost %q in:\n%s", keep, text)
		}
	}
	// Saving again is stable.
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(p); string(again) != text {
		t.Fatalf("not idempotent:\n%s\n---\n%s", text, again)
	}
}

func TestSaveNewAndBrokenFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.yaml")
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(p); err != nil || got != Defaults() {
		t.Fatalf("new: %+v %v", got, err)
	}
	if data, _ := os.ReadFile(p); !strings.HasPrefix(string(data), "# panal") {
		t.Fatalf("header: %s", data)
	}
	os.WriteFile(p, []byte("notify: [broken"), 0o600)
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(p); err != nil || got != Defaults() {
		t.Fatalf("broken replaced: %+v %v", got, err)
	}
}

func TestGitSettings(t *testing.T) {
	if d := Defaults(); d.Git.DefaultMode != ModeWorktree || d.Git.Editor != "" {
		t.Fatalf("defaults %+v", d.Git)
	}
	s, err := Parse([]byte("git:\n  editor: \" Visual Studio Code \"\n  default_mode: bogus\n"))
	if err != nil || s.Git.Editor != "Visual Studio Code" || s.Git.DefaultMode != ModeWorktree {
		t.Fatalf("got %+v %v", s.Git, err)
	}
	// An explicit "same" is kept.
	if s, _ := Parse([]byte("git:\n  default_mode: same\n")); s.Git.DefaultMode != ModeSame {
		t.Fatalf("same not kept: %q", s.Git.DefaultMode)
	}
	// An empty mode is saved as the default.
	p := filepath.Join(t.TempDir(), "settings.yaml")
	if err := Save(p, Settings{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(p); got.Git.DefaultMode != ModeWorktree {
		t.Fatalf("saved mode = %q", got.Git.DefaultMode)
	}
}

func TestUILanguage(t *testing.T) {
	if d := Defaults(); d.UI.Language != "auto" {
		t.Fatalf("default %q", d.UI.Language)
	}
	for in, want := range map[string]string{"en": "en", " ES ": "es", "auto": "auto", "fr": "auto", "": "auto"} {
		s, err := Parse([]byte("ui:\n  language: \"" + in + "\"\n"))
		if err != nil || s.UI.Language != want {
			t.Errorf("parse %q: got %q %v", in, s.UI.Language, err)
		}
	}
	// Saving edits ui.language in place, next to unknown ui keys.
	p := filepath.Join(t.TempDir(), "settings.yaml")
	os.WriteFile(p, []byte("ui:\n  theme: dark # mine\n  language: en\n"), 0o600)
	want := Defaults()
	want.UI.Language = "es"
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(p); err != nil || got != want {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "theme: dark # mine") || !strings.Contains(string(data), "language: es") {
		t.Fatalf("file:\n%s", data)
	}
	// Unknown values are written as auto.
	want.UI.Language = "klingon"
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(p); got.UI.Language != "auto" {
		t.Fatalf("saved %q", got.UI.Language)
	}
}

func TestSessionsRestore(t *testing.T) {
	if d := Defaults().Sessions.Restore; d != RestoreAsk {
		t.Fatalf("default restore = %q", d)
	}
	for in, want := range map[string]string{"always": RestoreAlways, " Never ": RestoreNever, "ask": RestoreAsk, "bogus": RestoreAsk} {
		s, err := Parse([]byte("sessions:\n  restore: " + in + "\n"))
		if err != nil || s.Sessions.Restore != want {
			t.Errorf("restore %q: got %q %v, want %q", in, s.Sessions.Restore, err, want)
		}
	}
	// Saving keeps the file's other keys and comments (in-place edit).
	p := filepath.Join(t.TempDir(), "settings.yaml")
	os.WriteFile(p, []byte("# mine\ncustom: 1\n"), 0o600)
	st := Defaults()
	st.Sessions.Restore = RestoreNever
	if err := Save(p, st); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "# mine") || !strings.Contains(string(b), "custom: 1") || !strings.Contains(string(b), "restore: never") {
		t.Fatalf("file:\n%s", b)
	}
}

func TestBrowserSettings(t *testing.T) {
	if b := Defaults().Browser; !b.Enabled || !b.Headless || b.Path != "" {
		t.Fatalf("defaults = %+v", b)
	}
	s, err := Parse([]byte("browser:\n  enabled: false\n  headless: false\n  path: ' /opt/chrome '\n"))
	if err != nil || s.Browser.Enabled || s.Browser.Headless || s.Browser.Path != "/opt/chrome" {
		t.Fatalf("parse = %+v, %v", s.Browser, err)
	}
	path := filepath.Join(t.TempDir(), "settings.yaml")
	in := Defaults()
	in.Browser = Browser{Enabled: false, Headless: true, Path: "/x/Chrome"}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil || out.Browser != in.Browser {
		t.Fatalf("round trip = %+v, %v", out.Browser, err)
	}
}

func TestMCPSettings(t *testing.T) {
	if !Defaults().MCP.Enabled {
		t.Fatal("the MCP server is on by default")
	}
	if s, err := Parse([]byte("mcp:\n  enabled: false\n")); err != nil || s.MCP.Enabled {
		t.Fatalf("parse = %+v, %v", s.MCP, err)
	}
	path := filepath.Join(t.TempDir(), "settings.yaml")
	in := Defaults()
	in.MCP.Enabled = false
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	if out, err := Load(path); err != nil || out.MCP.Enabled {
		t.Fatalf("round trip = %+v, %v", out.MCP, err)
	}
}

func TestHistoryDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.yaml")
	if st, _ := Load(p); !st.History.Enabled || st.History.Keep != DefaultHistoryKeep {
		t.Fatalf("defaults = %+v", st.History)
	}
	os.WriteFile(p, []byte("history:\n  keep: -3\n"), 0o600)
	if st, _ := Load(p); !st.History.Enabled || st.History.Keep != DefaultHistoryKeep {
		t.Fatalf("keep -3 = %+v", st.History)
	}
	os.WriteFile(p, []byte("history:\n  enabled: false\n  keep: 99999\n"), 0o600)
	if st, _ := Load(p); st.History.Enabled || st.History.Keep != 5000 {
		t.Fatalf("off, huge = %+v", st.History)
	}
}
