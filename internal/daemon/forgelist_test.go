package daemon_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elionaz/panal/internal/apierr"
	"github.com/elionaz/panal/internal/protocol"
)

// fakeGHPerBranch answers `pr list --head B` with a PR for the branches
// fix/a (merged, #21) and feat/b (open, #22), and none for others; the
// repository listings (`--state open|all` without --head) have them too.
// Every call is appended to the returned log file.
func fakeGHPerBranch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	a := `{"number":21,"title":"Fix a","state":"MERGED","url":"https://github.com/acme/app/pull/21","baseRefName":"main","headRefName":"fix/a","statusCheckRollup":[]}`
	b := `{"number":22,"title":"Feat b","state":"OPEN","url":"https://github.com/acme/app/pull/22","baseRefName":"main","headRefName":"feat/b","statusCheckRollup":[]}`
	script := `#!/bin/sh
printf '%s\n' "$*" >> '` + log + `'
case "$*" in
  *"--head fix/a "*) echo '[` + a + `]' ;;
  *"--head feat/b "*) echo '[` + b + `]' ;;
  *"--head "*) echo '[]' ;;
  "pr list"*"--state open"*) echo '[` + b + `]' ;;
  "pr list"*"--state all"*) echo '[` + b + `,` + a + `]' ;;
  "pr list"*) echo '[]' ;;
  "repo view"*) echo '{"squashMergeAllowed":true}' ;;
  *) echo "fake gh: $*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	testExtraPath.Store(dir)
	t.Cleanup(func() { testExtraPath.Store("") })
	return log
}

// A session that switches branches in its worktree: Changes follows the
// branch it is on, the worktree remembers the others, and forge.list
// shows the pull request of each.
func TestSessionPullRequestsAcrossBranches(t *testing.T) {
	calls := fakeGHPerBranch(t)
	c, _, stop := startDaemon(t)
	defer stop()
	ctx := ctxT(t)
	repo := tempRepo(t)
	gitT(t, repo, "remote", "add", "origin", "https://github.com/acme/app.git")
	gitT(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitT(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	proj, err := c.ProjectAdd(ctx, repo, "app")
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree})
	if err != nil {
		t.Fatal(err)
	}
	start := s.Branch
	ref := protocol.ForgeRef{SessionID: s.ID}
	// work on fix/a, then feat/b (the session's git info is refreshed on
	// each diff, which records the branch).
	for _, b := range []string{"fix/a", "feat/b"} {
		runIn(t, c, s.ID, "git switch -q -c "+b+" && git commit -q --allow-empty -m "+b)
		d, err := c.SessionDiff(ctx, s.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if d.Branch != b || d.StartBranch != start {
			t.Fatalf("diff on %s: branch %q, start %q", b, d.Branch, d.StartBranch)
		}
		// The branch is recorded when the session's git info refreshes.
		deadline := time.Now().Add(10 * time.Second)
		for !slices.Contains(d.Branches, b) {
			if time.Now().After(deadline) {
				t.Fatalf("branch %s not recorded: %v", b, d.Branches)
			}
			time.Sleep(100 * time.Millisecond)
			if d, err = c.SessionDiff(ctx, s.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}

	// The PR block follows the current branch.
	st, err := c.ForgeStatus(ctx, ref)
	if err != nil || st.Branch != "feat/b" || st.PR == nil || st.PR.Number != 22 {
		t.Fatalf("status: %+v, %v", st, err)
	}
	// Another branch it worked on, on request; not any branch.
	if st, err := c.ForgeStatus(ctx, protocol.ForgeRef{SessionID: s.ID, Branch: "fix/a"}); err != nil || st.PR == nil || st.PR.Number != 21 {
		t.Fatalf("status of fix/a: %+v, %v", st, err)
	}
	if _, err := c.ForgeStatus(ctx, protocol.ForgeRef{SessionID: s.ID, Branch: "someone/else"}); !hasCode(err, apierr.BranchMissing) {
		t.Fatalf("status of a foreign branch: %v", err)
	}
	// A pull request is only opened for the branch the folder is on.
	if _, err := c.ForgeCreate(ctx, protocol.ForgeCreateParams{ForgeRef: protocol.ForgeRef{SessionID: s.ID, Branch: "fix/a"}, Title: "x"}); !hasCode(err, apierr.ForgeHeadMismatch) {
		t.Fatalf("create for another branch: %v", err)
	}

	// The list: newest branch first, branches without a PR left out, from
	// two repository listings, not one call per branch.
	before, _ := os.ReadFile(calls)
	l, err := c.ForgeList(ctx, protocol.ForgeListParams{ForgeRef: ref, Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(calls)
	if made := strings.Split(strings.TrimSpace(string(after[len(before):])), "\n"); len(made) != 2 || strings.Contains(string(after[len(before):]), "--head") {
		t.Fatalf("forge.list calls = %q", made)
	}
	var got []int
	for _, it := range l.PRs {
		got = append(got, it.PR.Number)
	}
	if !slices.Equal(got, []int{22, 21}) || !l.PRs[0].Current || l.PRs[1].Current {
		t.Fatalf("list = %+v", l.PRs)
	}

	// Merge into the base merges the branch the worktree is on now, not
	// the one it was created on.
	res, err := c.WorktreeMerge(ctx, s.WorktreeID)
	if err != nil || res.Branch != "feat/b" {
		t.Fatalf("merge: %+v, %v", res, err)
	}
}
