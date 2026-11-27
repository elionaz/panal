package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/elionaz/panal/internal/apierr"
)

// Worktree is an entry of `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Head     string
	Branch   string // short name; "" when detached
	Bare     bool
	Detached bool
	Locked   bool
	Prunable bool // its directory is gone
}

// ListWorktrees lists the work trees of the repository containing dir.
func ListWorktrees(ctx context.Context, dir string) ([]Worktree, error) {
	out, err := git(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var cur *Worktree
	for _, l := range strings.Split(out, "\n") {
		key, val, _ := strings.Cut(l, " ")
		switch key {
		case "worktree":
			list = append(list, Worktree{Path: val})
			cur = &list[len(list)-1]
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		}
	}
	return list, nil
}

var (
	badRefChars = regexp.MustCompile(`[^A-Za-z0-9._/-]+`)
	dashes      = regexp.MustCompile(`-{2,}`)
	slashes     = regexp.MustCompile(`/{2,}`)
)

// SanitizeBranch turns s into a valid branch name (best effort; "" if
// nothing usable is left): spaces and odd characters become "-", no "..",
// no leading "-" or ".", no trailing "/", "." or ".lock".
func SanitizeBranch(s string) string {
	s = strings.TrimSpace(s)
	s = badRefChars.ReplaceAllString(s, "-")
	s = strings.ReplaceAll(s, "..", ".")
	s = slashes.ReplaceAllString(s, "/")
	s = dashes.ReplaceAllString(s, "-")
	parts := strings.Split(s, "/")
	var keep []string
	for _, p := range parts {
		p = strings.TrimLeft(p, ".-")
		p = strings.TrimSuffix(p, ".lock")
		p = strings.TrimRight(p, ".")
		if p != "" {
			keep = append(keep, p)
		}
	}
	s = strings.Join(keep, "/")
	if s == "@" || s == "HEAD" {
		return ""
	}
	return s
}

// UniqueBranch returns name, or name-2, name-3… if it is already taken.
func UniqueBranch(ctx context.Context, dir, name string) string {
	if !BranchExists(ctx, dir, name) {
		return name
	}
	for i := 2; ; i++ {
		c := name + "-" + strconv.Itoa(i)
		if !BranchExists(ctx, dir, c) {
			return c
		}
	}
}

// CheckBranchName validates name with `git check-ref-format --branch`.
func CheckBranchName(ctx context.Context, dir, name string) error {
	if name == "" {
		return apierr.New(apierr.BranchInvalid, map[string]any{"branch": name}, "empty branch name")
	}
	if _, err := git(ctx, dir, "check-ref-format", "--branch", name); err != nil {
		return apierr.New(apierr.BranchInvalid, map[string]any{"branch": name}, "invalid branch name: %q", name)
	}
	return nil
}

// AddWorktree runs `git worktree add <path> -b <branch> <base>` in the
// repository containing repo. The main work tree may be dirty: the new one
// starts from base's commit.
func AddWorktree(ctx context.Context, repo, path, branch, base string) error {
	if !IsRepo(ctx, repo) {
		return apierr.New(apierr.NotARepo, map[string]any{"dir": repo}, "%s: %w: worktree mode is not available", repo, ErrNotRepo)
	}
	if !HasCommits(ctx, repo) {
		return apierr.New(apierr.NoCommits, map[string]any{"dir": repo}, "the repository has no commits yet: make a first commit to use worktrees")
	}
	if _, err := ResolveCommit(ctx, repo, base); err != nil {
		return err
	}
	if err := CheckBranchName(ctx, repo, branch); err != nil {
		return err
	}
	if BranchExists(ctx, repo, branch) {
		return apierr.New(apierr.BranchExists, map[string]any{"branch": branch}, "branch %q already exists", branch)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("the worktree folder already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_, err := gitWrite(ctx, repo, "worktree", "add", "-b", branch, "--", path, base)
	return err
}

// RemoveOptions configure RemoveWorktree.
type RemoveOptions struct {
	// Force removes even with uncommitted changes (and, with
	// DeleteBranch, unmerged commits).
	Force bool
	// DeleteBranch also deletes the worktree's branch.
	DeleteBranch bool
	// Base is the branch unmerged commits are counted against.
	Base string
	// Upstream is the base's remote-tracking branch, if any: commits whose
	// changes it already has (a pull request squash- or rebase-merged on
	// the host, and the local base not pulled since) are not lost either.
	Upstream string
}

// LossError lists what removing a worktree would throw away.
type LossError struct {
	Path        string   `json:"path"`
	Branch      string   `json:"branch"`
	Base        string   `json:"base"`
	Uncommitted []string `json:"uncommitted,omitempty"`
	Unmerged    []Commit `json:"unmerged,omitempty"`
}

// ErrorCode classifies the refusal (apierr.Coded): uncommitted files win.
func (e *LossError) ErrorCode() string {
	if len(e.Uncommitted) > 0 {
		return apierr.WorktreeDirty
	}
	return apierr.WorktreeUnmerged
}

// ErrorData is the LossError itself (its JSON form).
func (e *LossError) ErrorData() any { return e }

func (e *LossError) Error() string {
	var parts []string
	if n := len(e.Uncommitted); n > 0 {
		parts = append(parts, fmt.Sprintf("%d uncommitted %s (%s)", n, plural(n, "file", "files"), preview(e.Uncommitted, 5)))
	}
	if n := len(e.Unmerged); n > 0 {
		subjects := make([]string, 0, len(e.Unmerged))
		for _, c := range e.Unmerged {
			subjects = append(subjects, c.Short+" "+c.Subject)
		}
		parts = append(parts, fmt.Sprintf("%d %s on %s not merged into %s (%s)", n, plural(n, "commit", "commits"), e.Branch, e.Base, preview(subjects, 3)))
	}
	return "worktree not removed: it would lose " + strings.Join(parts, " and ") + "; use force to remove it anyway"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func preview(list []string, n int) string {
	if len(list) <= n {
		return strings.Join(list, ", ")
	}
	return strings.Join(list[:n], ", ") + fmt.Sprintf(", … +%d", len(list)-n)
}

// Losses reports what removing the worktree at path would lose: its
// uncommitted changes and, if the branch is deleted too, the commits not
// merged into base (by ancestry or, after a squash merge, by content). It returns nil when nothing would be lost.
func Losses(ctx context.Context, repo, path, branch string, o RemoveOptions) (*LossError, error) {
	le := &LossError{Path: path, Branch: branch, Base: o.Base}
	if _, err := os.Stat(path); err == nil {
		st, err := GetStatus(ctx, path)
		if err != nil {
			return nil, err
		}
		le.Uncommitted = st.Files
	}
	if o.DeleteBranch && branch != "" && o.Base != "" {
		if BranchExists(ctx, repo, branch) {
			head := "refs/heads/" + branch
			cs, err := CommitsAhead(ctx, repo, o.Base, head)
			if err != nil {
				return nil, err
			}
			// Commits whose changes base already has (a squash or
			// rebase merge on the host) are not lost with the branch.
			if len(cs) > 0 {
				if mb, err := MergeBase(ctx, repo, o.Base, head); err == nil && AlreadyIn(ctx, repo, o.Base, mb, head) {
					cs = nil
				}
			}
			if len(cs) > 0 && o.Upstream != "" && o.Upstream != o.Base {
				if mb, err := MergeBase(ctx, repo, o.Upstream, head); err == nil && AlreadyIn(ctx, repo, o.Upstream, mb, head) {
					cs = nil
				}
			}
			le.Unmerged = cs
		}
	}
	if len(le.Uncommitted) == 0 && len(le.Unmerged) == 0 {
		return nil, nil
	}
	return le, nil
}

// RemoveWorktree removes the linked worktree at path from the repository
// containing repo (a directory of the main work tree). Without o.Force it
// refuses with a *LossError when that would lose uncommitted changes (or,
// with o.DeleteBranch, unmerged commits). A worktree whose directory is
// already gone is just pruned.
func RemoveWorktree(ctx context.Context, repo, path, branch string, o RemoveOptions) error {
	if !o.Force {
		le, err := Losses(ctx, repo, path, branch, o)
		if err != nil {
			return err
		}
		if le != nil {
			return le
		}
	}
	if _, err := os.Stat(path); err == nil {
		args := []string{"worktree", "remove"}
		if o.Force {
			args = append(args, "--force", "--force") // twice: also locked ones
		}
		if _, err := gitWrite(ctx, repo, append(args, "--", path)...); err != nil {
			return err
		}
	}
	if err := Prune(ctx, repo); err != nil {
		return err
	}
	if o.DeleteBranch && branch != "" && BranchExists(ctx, repo, branch) {
		// Unmerged commits were checked above (against Base, which is
		// what matters here, not whatever HEAD is): -D is safe.
		if _, err := gitWrite(ctx, repo, "branch", "-D", "--", branch); err != nil {
			return err
		}
	}
	return nil
}

// Prune drops administrative data of worktrees whose directory is gone.
func Prune(ctx context.Context, repo string) error {
	_, err := gitWrite(ctx, repo, "worktree", "prune")
	return err
}

// MergeResult describes a successful merge.
type MergeResult struct {
	Base    string `json:"base"`
	Branch  string `json:"branch"`
	Commits int    `json:"commits"`
	Output  string `json:"output,omitempty"`
	// Uncommitted: changes left in the worktree that were not merged.
	Uncommitted int `json:"uncommitted,omitempty"`
}

// Merge merges branch into base with `git merge --no-ff` in the main work
// tree, only when that tree is checked out on base and has no uncommitted
// changes to tracked files (no auto-stash). A conflicting merge is aborted
// and reported. worktree (optional) is the branch's worktree, to report
// changes that were not committed (and so are not merged).
func Merge(ctx context.Context, repo, branch, base, worktree string) (MergeResult, error) {
	res := MergeResult{Base: base, Branch: branch}
	main, err := MainWorktree(ctx, repo)
	if err != nil {
		return res, err
	}
	cur, err := CurrentBranch(ctx, main)
	if err != nil {
		return res, err
	}
	if cur != base {
		where := "has a detached HEAD"
		if cur != "" {
			where = "is on branch " + cur
		}
		return res, apierr.New(apierr.MergeWrongBranch, map[string]any{"dir": main, "current": cur, "base": base},
			"cannot merge: the main folder (%s) %s, not %s; switch it to %s and try again", main, where, base, base)
	}
	st, err := GetStatus(ctx, main)
	if err != nil {
		return res, err
	}
	if n := st.Staged + st.Unstaged + st.Conflicts; n > 0 {
		return res, apierr.New(apierr.MergeMainDirty, map[string]any{"dir": main, "count": n},
			"cannot merge: the main folder (%s) has %d uncommitted %s; commit or stash first (panal does not stash automatically)", main, n, plural(n, "change", "changes"))
	}
	if !BranchExists(ctx, main, branch) {
		return res, apierr.New(apierr.BranchMissing, map[string]any{"branch": branch}, "branch %q no longer exists", branch)
	}
	// Merging into a base that is behind its remote would make a local
	// merge commit the remote never gets (and that pulling then has to
	// reconcile): the base has to be pulled first.
	if up := Upstream(ctx, main, base); up != "" {
		if behind, err := AheadCount(ctx, main, base, up); err == nil && behind > 0 {
			return res, apierr.New(apierr.MergeBaseBehind, map[string]any{"dir": main, "base": base, "upstream": ShortRef(up), "count": behind},
				"cannot merge: %s is %d %s behind %s; pull it first (git -C %s pull --ff-only)", base, behind, plural(behind, "commit", "commits"), ShortRef(up), main)
		}
	}
	n, err := AheadCount(ctx, main, base, "refs/heads/"+branch)
	if err != nil {
		return res, err
	}
	res.Commits = n
	if worktree != "" {
		if wst, err := GetStatus(ctx, worktree); err == nil {
			res.Uncommitted = wst.Changed()
		}
	}
	if n == 0 {
		return res, apierr.New(apierr.MergeNothing, map[string]any{"branch": branch, "base": base},
			"nothing to merge: %s has no commits that are not already in %s", branch, base)
	}
	out, err := gitWrite(ctx, main, "merge", "--no-ff", "--no-edit", "-m", fmt.Sprintf("Merge branch '%s' (panal)", branch), "--", branch)
	if err != nil {
		conflicts, _ := git(ctx, main, "diff", "--name-only", "--diff-filter=U")
		_, _ = gitWrite(ctx, main, "merge", "--abort")
		if c := splitLines(conflicts); len(c) > 0 {
			return res, apierr.New(apierr.MergeConflict, map[string]any{"branch": branch, "base": base, "files": c},
				"merging %s into %s conflicts (%s): the merge was aborted; resolve it by hand", branch, base, preview(c, 5))
		}
		return res, err
	}
	res.Output = out
	return res, nil
}
