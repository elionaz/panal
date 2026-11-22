package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elionaz/panal/internal/fileformat"
)

func entry(id, project string, ended time.Time) Entry {
	return Entry{ID: id, ProjectID: project, ProjectName: "Proj " + project, ProfileID: "claude", Agent: "Claude Code",
		Cwd: "/tmp/x", Branches: []string{"agent/" + id}, CreatedAt: ended.Add(-time.Hour), EndedAt: ended}
}

func TestSaveGetListDelete(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	e := entry("aaaa1111", "p1", now)
	e.Prompts = []string{"fix the login", "my key is sk-ant-abcdefghijklmnop please"}
	if err := s.Save(e, []string{"$ go test", "token=hunter2hunter2", "ok"}, 0); err != nil {
		t.Fatal(err)
	}
	got, text, err := s.Get("aaaa1111")
	if err != nil {
		t.Fatal(err)
	}
	if got.Prompts[1] != "my key is [redacted] please" || strings.Contains(text, "hunter2") || !strings.Contains(text, "$ go test\n") {
		t.Fatalf("redaction: %q / %q", got.Prompts, text)
	}
	if got.TextBytes != len(text) {
		t.Errorf("TextBytes = %d, text %d", got.TextBytes, len(text))
	}
	for _, f := range []string{s.jsonPath("aaaa1111"), s.textPath("aaaa1111")} {
		if fi, err := os.Stat(f); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v %v", f, fi, err)
		}
	}
	if err := s.Save(entry("bbbb2222", "p1", now.Add(time.Minute)), nil, 0); err != nil {
		t.Fatal(err)
	}
	if l := s.List(); len(l) != 2 || l[0].ID != "bbbb2222" {
		t.Fatalf("list = %+v", l)
	}
	if err := s.Delete("bbbb2222"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("bbbb2222"); err != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
	if _, _, err := s.Get("../etc"); err != ErrNotFound {
		t.Fatalf("bad id: %v", err)
	}
	if err := s.Save(Entry{ID: "../x"}, nil, 0); err == nil {
		t.Fatal("saved a bad id")
	}
}

func TestKeepAndTextLimit(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now().UTC()
	for i, id := range []string{"aaaa0001", "aaaa0002", "aaaa0003"} {
		if err := s.Save(entry(id, "p", now.Add(time.Duration(i)*time.Minute)), nil, 2); err != nil {
			t.Fatal(err)
		}
	}
	if l := s.List(); len(l) != 2 || l[1].ID != "aaaa0002" {
		t.Fatalf("after keep 2: %+v", l)
	}
	big := make([]string, 0, 40000)
	for i := range 40000 {
		big = append(big, strings.Repeat("x", 20)+string(rune('a'+i%26)))
	}
	big = append(big, "the end")
	if err := s.Save(entry("cccc0001", "p", now.Add(time.Hour)), big, 0); err != nil {
		t.Fatal(err)
	}
	_, text, _ := s.Get("cccc0001")
	if len(text) > MaxText || !strings.HasSuffix(text, "the end") || strings.HasPrefix(text, "\n") {
		t.Fatalf("text %d bytes, tail %q", len(text), text[len(text)-10:])
	}
}

func TestSearch(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now().UTC()
	a := entry("aaaa0001", "p1", now)
	a.Name = "login fix"
	a.Prompts = []string{"Fix the OAuth redirect", "then run the tests"}
	s.Save(a, []string{"compiling…", "redirect_uri mismatch in callback.go", "done"}, 0)
	b := entry("bbbb0001", "p2", now.Add(time.Minute))
	b.Prompts = []string{"add dark mode"}
	s.Save(b, []string{"wrote theme.css", "Redirect not touched"}, 0)

	hits := s.Search(Query{Text: "redirect"})
	if len(hits) != 2 || hits[0].Entry.ID != "bbbb0001" {
		t.Fatalf("redirect: %+v", hits)
	}
	if h := hits[1]; h.Count != 2 || h.Snippets[0].Source != "prompt" || h.Snippets[1].Source != "text" {
		t.Fatalf("a's snippets: %+v", h)
	}
	sn := hits[1].Snippets[0]
	if m := sn.Matches[0]; sn.Line[m[0]:m[1]] != "redirect" && sn.Line[m[0]:m[1]] != "Redirect" {
		t.Fatalf("match range %v in %q", m, sn.Line)
	}
	// Every word must appear, anywhere in the entry.
	if hits := s.Search(Query{Text: "oauth callback"}); len(hits) != 1 || hits[0].Entry.ID != "aaaa0001" {
		t.Fatalf("two words: %+v", hits)
	}
	if hits := s.Search(Query{Text: "oauth theme"}); len(hits) != 0 {
		t.Fatalf("words from two entries matched: %+v", hits)
	}
	// Names and branches count too; project filter; empty query lists.
	if hits := s.Search(Query{Text: "LOGIN"}); len(hits) != 1 || hits[0].Snippets[0].Source != "info" {
		t.Fatalf("name: %+v", hits)
	}
	if hits := s.Search(Query{Text: "agent/bbbb0001"}); len(hits) != 1 {
		t.Fatalf("branch: %+v", hits)
	}
	if hits := s.Search(Query{Text: "redirect", ProjectID: "p1"}); len(hits) != 1 {
		t.Fatalf("project filter: %+v", hits)
	}
	if hits := s.Search(Query{Limit: 1}); len(hits) != 1 || hits[0].Entry.ID != "bbbb0001" || len(hits[0].Snippets) != 0 {
		t.Fatalf("empty query: %+v", hits)
	}
}

func TestSnippetCut(t *testing.T) {
	line := strings.Repeat("a", 1000) + "needle" + strings.Repeat("b", 1000)
	sn := snippet("text", line, [][2]int{{1000, 1006}})
	if !strings.HasPrefix(sn.Line, "…") || !strings.HasSuffix(sn.Line, "…") || len(sn.Matches) != 1 {
		t.Fatalf("cut = %d bytes, %v", len(sn.Line), sn.Matches)
	}
	if m := sn.Matches[0]; sn.Line[m[0]:m[1]] != "needle" {
		t.Fatalf("range %v", m)
	}
}

// Entries say their format. An unversioned one (before it was versioned)
// is read as format 1; a newer panal's entry is left out of the list (so
// it is never pruned) and refused by Get, and the file is left alone.
func TestEntryFormat(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Save(Entry{ID: "aa11", EndedAt: time.Now()}, []string{"x"}, 0); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "aa11.json"))
	if !strings.Contains(string(data), `"version": 1`) {
		t.Fatalf("saved entry: %s", data)
	}
	os.WriteFile(filepath.Join(dir, "bb22.json"), []byte(`{"id":"bb22","endedAt":"2026-01-01T00:00:00Z"}`), 0o600)
	newer := []byte(`{"version":2,"id":"cc33","endedAt":"2027-01-01T00:00:00Z"}`)
	os.WriteFile(filepath.Join(dir, "cc33.json"), newer, 0o600)

	var ids []string
	for _, e := range s.List() {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, ",") != "aa11,bb22" {
		t.Fatalf("list = %v", ids)
	}
	if _, _, err := s.Get("cc33"); !fileformat.IsTooNew(err) {
		t.Fatalf("get newer: %v", err)
	}
	s.Save(Entry{ID: "dd44", EndedAt: time.Now()}, []string{"y"}, 1) // prunes down to 1
	if got, _ := os.ReadFile(filepath.Join(dir, "cc33.json")); string(got) != string(newer) {
		t.Fatalf("newer entry touched: %q", got)
	}
}
