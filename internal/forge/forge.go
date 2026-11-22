// Package forge talks to the code host of a repository (GitHub, GitLab)
// through its official CLI (gh, glab), using the account the user already
// logged in with. panal keeps no tokens: no CLI or no login means the
// integration is off and the app falls back to pushing and opening the
// host's page.
package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elionaz/panal/internal/apierr"
	"github.com/elionaz/panal/internal/gitx"
)

// Kinds.
const (
	GitHub = "github"
	GitLab = "gitlab"
)

// PR states.
const (
	StateOpen   = "open"
	StateDraft  = "draft"
	StateMerged = "merged"
	StateClosed = "closed"
)

// Review decisions (empty: none / not reported).
const (
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes_requested"
	ReviewRequired         = "review_required"
)

// Checks states.
const (
	ChecksPassing = "passing"
	ChecksFailing = "failing"
	ChecksPending = "pending"
	ChecksNone    = "none"
)

// PR is a pull request (GitHub) or merge request (GitLab).
type PR struct {
	Number int    `json:"number"` // GitLab: the iid
	Title  string `json:"title"`
	State  string `json:"state"` // State*
	URL    string `json:"url"`
	Base   string `json:"base"`
	Head   string `json:"head"`
	Review string `json:"review,omitempty"` // Review*
	// Conflicts: the host says it does not merge cleanly.
	Conflicts bool `json:"conflicts,omitempty"`
	// Blocked: the host refuses the merge for now (rules, approvals…).
	Blocked bool   `json:"blocked,omitempty"`
	Checks  Checks `json:"checks"`
}

// Checks summarises CI (GitHub checks and statuses, GitLab pipeline).
type Checks struct {
	State   string   `json:"state"` // Checks*
	Total   int      `json:"total"`
	Passed  int      `json:"passed"`
	Failed  int      `json:"failed"`
	Pending int      `json:"pending"`
	Failing []string `json:"failing,omitempty"` // names of the failed ones
}

// Comment is a conversation comment, a review (approve / request changes
// / commented with text) or a comment on a line of the diff.
type Comment struct {
	Kind   string    `json:"kind"` // comment | review | line
	Author string    `json:"author"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
	URL    string    `json:"url,omitempty"`
	// Review: its state (approved, changes_requested, commented, dismissed).
	Review string `json:"review,omitempty"`
	// Line comments: the file and line (new side, else old side).
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
}

// MergeResult is what a merge did.
type MergeResult struct {
	// Merged: the host reports the pull request merged; otherwise it only
	// queued the merge (it happens when checks pass or the queue gets to it).
	Merged bool
	// Warning: something after the merge failed (deleting the branch).
	Warning string
}

// CreateParams opens a pull request from Head into Base.
type CreateParams struct {
	Head  string
	Base  string
	Title string
	Body  string
	Draft bool
}

// Forge is a code host reached through its CLI.
// Lister is a Forge that finds the pull requests of many branches at
// once (one or two host calls instead of one per branch). Statuses maps
// each branch that has one to its pull request, chosen as Status does;
// complete reports that the host's whole history was searched, so a
// branch left out has none (else the caller asks Status for those).
type Lister interface {
	Statuses(ctx context.Context, branches []string) (prs map[string]*PR, complete bool, err error)
}

type Forge interface {
	Kind() string
	Tool() string
	// Status is the pull request of branch (the newest open one, else the
	// newest at all), nil when there is none.
	Status(ctx context.Context, branch string) (*PR, error)
	Comments(ctx context.Context, number int) ([]Comment, error)
	// MergeMethods are the ones the repository allows (squash, merge,
	// rebase), the preferred first.
	MergeMethods(ctx context.Context) ([]string, error)
	// Merge merges the pull request with method, refusing (apierr.
	// ForgeHeadMismatch) unless its head is branch head of this very
	// repository (not a fork's branch of the same name). deleteBranch
	// removes the branch on the host once the merge is done, never the
	// local one; a merge the host only queued (merge queue, GitLab's
	// auto-merge) keeps it.
	Merge(ctx context.Context, number int, method, head string, deleteBranch bool) (MergeResult, error)
	Create(ctx context.Context, p CreateParams) error
}

// For is the forge of the repository remote points to (remoteURL), run
// with PATH path. It fails with apierr.ForgeUnsupported for other hosts
// and apierr.ForgeCLIMissing when the CLI is not installed.
func For(remoteURL, dir, path string) (Forge, error) {
	r, ok := gitx.RemoteOf(remoteURL)
	switch {
	case ok && r.GitHub:
		c, err := newCLI("gh", dir, path, r)
		if err != nil {
			return nil, err
		}
		return &github{c}, nil
	case ok && r.GitLab:
		c, err := newCLI("glab", dir, path, r)
		if err != nil {
			return nil, err
		}
		return &gitlab{c}, nil
	}
	host := ""
	if ok {
		host = r.Host
	}
	return nil, apierr.New(apierr.ForgeUnsupported, map[string]any{"host": host},
		"pull requests from panal need a GitHub or GitLab remote (%s is neither)", cmpOr(host, remoteURL))
}

// cli runs one tool in a repository.
type cli struct {
	tool   string
	exe    string
	dir    string
	env    []string
	remote gitx.Remote
}

// maxOutput bounds what a command may print (JSON of a big PR included).
const maxOutput = 8 << 20

func newCLI(tool, dir, path string, r gitx.Remote) (*cli, error) {
	exe := lookPath(tool, path)
	if exe == "" {
		return nil, apierr.New(apierr.ForgeCLIMissing, map[string]any{"tool": tool, "host": r.Host, "install": "brew install " + tool},
			"%s is not installed: install it (brew install %s) and log in (%s auth login) to see pull requests here", tool, tool, tool)
	}
	env := append(os.Environ(), "PATH="+path,
		// Never prompt or page: there is no terminal to answer.
		"GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "GLAB_NO_PROMPT=1", "NO_PROMPT=1",
		"PAGER=cat", "GH_PAGER=cat", "GLAB_PAGER=cat", "NO_COLOR=1", "CLICOLOR=0")
	return &cli{tool: tool, exe: exe, dir: dir, env: env, remote: r}, nil
}

// lookPath finds tool in path (a PATH list), not in the process' PATH: a
// daemon started from the app has a minimal one.
func lookPath(tool, path string) string {
	for _, d := range filepath.SplitList(path) {
		if d == "" {
			continue
		}
		p := filepath.Join(d, tool)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

func headMismatch(number int, want, got string) error {
	return apierr.New(apierr.ForgeHeadMismatch, map[string]any{"number": number, "branch": want, "head": got},
		"pull request %d is not this repository's %s (its head is %s): reload and try again", number, want, got)
}

// run executes the tool and returns its stdout. Failures become coded
// errors: not logged in (ForgeNotAuthed) or anything else (ForgeFailed,
// with the tool's own message).
func (c *cli) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.exe, args...)
	cmd.Dir, cmd.Env = c.dir, c.env
	cmd.Stdin = nil
	var out, errb limited
	out.max, errb.max = maxOutput, 64<<10
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err == nil {
		return out.b.Bytes(), nil
	}
	msg := strings.TrimSpace(errb.b.String())
	if msg == "" {
		msg = strings.TrimSpace(out.b.String())
	}
	if ctx.Err() != nil {
		msg = cmpOr(msg, ctx.Err().Error())
	}
	if notAuthed(msg) {
		return nil, apierr.New(apierr.ForgeNotAuthed, map[string]any{"tool": c.tool, "host": c.remote.Host},
			"%s is not logged in to %s: run `%s auth login` in a terminal", c.tool, c.remote.Host, c.tool)
	}
	return nil, apierr.New(apierr.ForgeFailed, map[string]any{"tool": c.tool, "stderr": firstLines(msg, 6)},
		"%s %s failed: %s", c.tool, args[0], firstLines(msg, 6))
}

func notAuthed(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{"auth login", "not logged in", "authentication required", "bad credentials", "401 unauthorized", "http 401"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// limited is a bytes.Buffer that stops growing at max (the rest is dropped).
type limited struct {
	b   bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	if room := l.max - l.b.Len(); room > 0 {
		if len(p) > room {
			l.b.Write(p[:room])
		} else {
			l.b.Write(p)
		}
	}
	return len(p), nil
}

// decodeArrays reads one or more JSON arrays written back to back (what
// `gh api --paginate` / `glab api --paginate` print, one per page) into a
// single slice.
func decodeArrays[T any](b []byte) ([]T, error) {
	var all []T
	dec := json.NewDecoder(bytes.NewReader(b))
	for dec.More() {
		var page []T
		if err := dec.Decode(&page); err != nil {
			return nil, err
		}
		all = append(all, page...)
	}
	return all, nil
}

// summarize counts check results by state.
func summarize(states []checkResult) Checks {
	c := Checks{Total: len(states)}
	for _, s := range states {
		switch s.state {
		case ChecksPassing:
			c.Passed++
		case ChecksFailing:
			c.Failed++
			c.Failing = append(c.Failing, s.name)
		default:
			c.Pending++
		}
	}
	switch {
	case c.Total == 0:
		c.State = ChecksNone
	case c.Failed > 0:
		c.State = ChecksFailing
	case c.Pending > 0:
		c.State = ChecksPending
	default:
		c.State = ChecksPassing
	}
	return c
}

type checkResult struct {
	name  string
	state string // ChecksPassing | ChecksFailing | ChecksPending
}
