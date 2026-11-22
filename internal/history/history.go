// Package history keeps what is left of agent sessions after they end: a
// record per session (who, where, when, the user's prompts) and its
// terminal output as plain text, so past sessions can be searched, read
// and resumed. Everything lives under one directory, readable only by the
// user: <id>.json (Entry) and <id>.txt (the text). Secrets that look like
// keys or tokens are masked before anything is written (internal/redact).
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/elionaz/panal/internal/atomicfile"
	"github.com/elionaz/panal/internal/fileformat"
	"github.com/elionaz/panal/internal/redact"
)

// Limits.
const (
	// MaxText is how much terminal text an entry keeps (its end).
	MaxText = 512 << 10
	// MaxPrompts and MaxPromptRunes bound the prompts kept.
	MaxPrompts     = 300
	MaxPromptRunes = 4000
	// DefaultKeep is how many entries are kept when the setting is unset.
	DefaultKeep = 300
	// maxSnippets per hit, maxSnippetRunes per snippet line.
	maxSnippets     = 3
	maxSnippetRunes = 200
)

// Resume is how the agent's own conversation can be reopened.
type Resume struct {
	Agent     string `json:"agent"`
	SessionID string `json:"sessionId"`
}

// EntryFormat is the newest format of <id>.json this build reads (see
// internal/fileformat). Entries written before it was versioned have no
// version: they are format 1.
const EntryFormat = 1

// Entry describes one ended session.
type Entry struct {
	// Version is the entry's format (EntryFormat when saved).
	Version     int    `json:"version"`
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	ProfileID   string `json:"profileId"`
	// Agent is the profile's display name; Name the session's own name.
	Agent string `json:"agent"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
	Cwd   string `json:"cwd"`
	// Mode is same | worktree; WorktreeID names the worktree (it may be
	// gone by now).
	Mode       string `json:"mode,omitempty"`
	WorktreeID string `json:"worktreeId,omitempty"`
	// Branches the session worked on, the last one first.
	Branches  []string  `json:"branches,omitempty"`
	Base      string    `json:"base,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	EndedAt   time.Time `json:"endedAt"`
	ExitCode  int       `json:"exitCode"`
	Resume    *Resume   `json:"resume,omitempty"`
	// Prompts the user typed (agents with a readable transcript), oldest
	// first, already redacted.
	Prompts []string `json:"prompts,omitempty"`
	// TextBytes is the size of the kept terminal text.
	TextBytes int `json:"textBytes"`
}

// Store is the history directory.
type Store struct {
	dir string
	mu  sync.Mutex
}

// New returns the store in dir (created on first write).
func New(dir string) *Store { return &Store{dir: dir} }

var idRe = regexp.MustCompile(`^[a-z0-9]{4,32}$`)

func validID(id string) bool { return idRe.MatchString(id) }

func (s *Store) jsonPath(id string) string { return filepath.Join(s.dir, id+".json") }
func (s *Store) textPath(id string) string { return filepath.Join(s.dir, id+".txt") }

// Save writes e and its terminal text (lines, oldest first; only the last
// MaxText bytes are kept). Prompts and text are redacted here, and e is
// replaced if it already exists. keep > 0 then drops the oldest entries
// beyond keep.
func (s *Store) Save(e Entry, lines []string, keep int) error {
	if !validID(e.ID) {
		return fmt.Errorf("invalid session id %q", e.ID)
	}
	text := redact.String(strings.Join(lines, "\n"))
	if len(text) > MaxText {
		text = text[len(text)-MaxText:]
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
		text = strings.ToValidUTF8(text, "")
	}
	e.TextBytes = len(text)
	if len(e.Prompts) > MaxPrompts {
		e.Prompts = e.Prompts[len(e.Prompts)-MaxPrompts:]
	}
	ps := make([]string, 0, len(e.Prompts))
	for _, p := range e.Prompts {
		ps = append(ps, redact.String(truncRunes(p, MaxPromptRunes)))
	}
	e.Prompts = ps
	e.Version = EntryFormat
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := atomicfile.Replace(s.textPath(e.ID), []byte(text), 0o600); err != nil {
		return err
	}
	if err := atomicfile.Replace(s.jsonPath(e.ID), data, 0o600); err != nil {
		return err
	}
	if keep > 0 {
		s.pruneLocked(keep)
	}
	return nil
}

// List returns every entry, the most recently ended first. Unreadable
// files are skipped.
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *Store) listLocked() []Entry {
	ents, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, de := range ents {
		id, ok := strings.CutSuffix(de.Name(), ".json")
		if !ok || !validID(id) {
			continue
		}
		data, err := os.ReadFile(s.jsonPath(id))
		if err != nil {
			continue
		}
		var e Entry
		// A newer panal's entry is left out (and so never pruned).
		if json.Unmarshal(data, &e) != nil || e.ID != id || fileformat.Check(s.jsonPath(id), e.Version, EntryFormat) != nil {
			continue
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b Entry) int { return b.EndedAt.Compare(a.EndedAt) })
	return out
}

// ErrNotFound: no entry with that id.
var ErrNotFound = errors.New("no such session in the history")

// Get returns entry id and its terminal text.
func (s *Store) Get(id string) (Entry, string, error) {
	if !validID(id) {
		return Entry{}, "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.jsonPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return Entry{}, "", ErrNotFound
	}
	if err != nil {
		return Entry{}, "", err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, "", err
	}
	if err := fileformat.Check(s.jsonPath(id), e.Version, EntryFormat); err != nil {
		return Entry{}, "", err
	}
	text, err := os.ReadFile(s.textPath(id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Entry{}, "", err
	}
	return e, string(text), nil
}

// Delete removes entry id.
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.jsonPath(id)); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return s.removeLocked(id)
}

func (s *Store) removeLocked(id string) error {
	err := os.Remove(s.jsonPath(id))
	if terr := os.Remove(s.textPath(id)); terr != nil && !errors.Is(terr, os.ErrNotExist) && err == nil {
		err = terr
	}
	return err
}

func (s *Store) pruneLocked(keep int) {
	list := s.listLocked()
	for _, e := range list[min(keep, len(list)):] {
		_ = s.removeLocked(e.ID)
	}
}

// Snippet is a matching line: where it came from (prompt, text, info),
// the line, and the byte ranges of the matches in it.
type Snippet struct {
	Source  string   `json:"source"`
	Line    string   `json:"line"`
	Matches [][2]int `json:"matches"`
}

// Hit is an entry that matches a search.
type Hit struct {
	Entry    Entry     `json:"entry"`
	Snippets []Snippet `json:"snippets"`
	// Count is how many lines matched in all.
	Count int `json:"count"`
}

// Query selects entries: every word of Text must appear (case-insensitive)
// somewhere in the entry: its name, agent, project, branches, prompts or
// text. An empty Text lists the most recent entries.
type Query struct {
	Text      string
	ProjectID string
	Limit     int
}

// Search returns the entries matching q, the most recently ended first.
func (s *Store) Search(q Query) []Hit {
	words := strings.Fields(strings.ToLower(q.Text))
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	var out []Hit
	for _, e := range s.List() {
		if q.ProjectID != "" && e.ProjectID != q.ProjectID {
			continue
		}
		h, ok := s.match(e, words)
		if !ok {
			continue
		}
		out = append(out, h)
		if len(out) == limit {
			break
		}
	}
	return out
}

func (s *Store) match(e Entry, words []string) (Hit, bool) {
	h := Hit{Entry: e, Snippets: []Snippet{}}
	if len(words) == 0 {
		return h, true
	}
	seen := make([]bool, len(words))
	add := func(source, line string) {
		lower := strings.ToLower(line)
		var ms [][2]int
		for i, w := range words {
			for off := 0; ; {
				j := strings.Index(lower[off:], w)
				if j < 0 {
					break
				}
				seen[i] = true
				ms = append(ms, [2]int{off + j, off + j + len(w)})
				off += j + len(w)
			}
		}
		if ms == nil {
			return
		}
		if len(lower) != len(line) {
			ms = nil // case folding changed the byte offsets: no highlight
		}
		h.Count++
		if len(h.Snippets) < maxSnippets {
			h.Snippets = append(h.Snippets, snippet(source, line, ms))
		}
	}
	info := strings.Join(append([]string{e.Name, e.Agent, e.ProjectName, e.Model}, e.Branches...), " ")
	add("info", info)
	for _, p := range e.Prompts {
		for _, l := range strings.Split(p, "\n") {
			add("prompt", l)
		}
	}
	s.mu.Lock()
	f, err := os.Open(s.textPath(e.ID))
	s.mu.Unlock()
	if err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), MaxText+1)
		for sc.Scan() {
			if b := sc.Bytes(); containsAny(b, words) {
				add("text", sc.Text())
			}
		}
		f.Close()
	}
	for _, ok := range seen {
		if !ok {
			return Hit{}, false
		}
	}
	return h, true
}

// containsAny is a cheap pre-filter on a text line (ASCII case folding is
// enough here: add does the real matching).
func containsAny(line []byte, words []string) bool {
	lower := bytes.ToLower(line)
	for _, w := range words {
		if bytes.Contains(lower, []byte(w)) {
			return true
		}
	}
	return false
}

// snippet cuts line around its first match so it fits maxSnippetRunes,
// shifting the match ranges accordingly.
func snippet(source, line string, ms [][2]int) Snippet {
	slices.SortFunc(ms, func(a, b [2]int) int { return a[0] - b[0] })
	maxBytes := maxSnippetRunes * 2
	start := 0
	if len(line) > maxBytes && len(ms) > 0 && ms[0][0] > maxBytes/3 {
		start = ms[0][0] - maxBytes/3
		for start < len(line) && !utf8.RuneStart(line[start]) {
			start++
		}
	}
	end := len(line)
	if end-start > maxBytes {
		end = start + maxBytes
		for end > start && !utf8.RuneStart(line[end]) {
			end--
		}
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(line) {
		suffix = "…"
	}
	out := Snippet{Source: source, Line: prefix + line[start:end] + suffix, Matches: [][2]int{}}
	for _, m := range ms {
		if m[0] >= start && m[1] <= end {
			out.Matches = append(out.Matches, [2]int{m[0] - start + len(prefix), m[1] - start + len(prefix)})
		}
	}
	return out
}

func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
