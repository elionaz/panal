package daemon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/elionaz/panal/internal/apierr"
	"github.com/elionaz/panal/internal/client"
	"github.com/elionaz/panal/internal/daemon"
	"github.com/elionaz/panal/internal/protocol"
)

// TestMain isolates git (and the shells the tests spawn) from the user's
// configuration: no system config, an empty global one, a throwaway HOME.
func TestMain(m *testing.M) {
	if os.Getenv("PANAL_FAKE_CHROME") != "" {
		fakeChrome() // the test binary as the agents' browser (browser_test.go)
		return
	}
	home, err := os.MkdirTemp("", "agt-home")
	if err != nil {
		panic(err)
	}
	global := filepath.Join(home, "gitconfig")
	_ = os.WriteFile(global, nil, 0o600)
	os.Setenv("HOME", home)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", global)
	// Test daemons never post real desktop notifications (they also get a
	// no-op or recording Notifier).
	os.Setenv("PANAL_NOTIFY", "off")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// tempRepo is a throwaway repository on main with one commit.
func tempRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "-q", "-b", "main")
	gitT(t, dir, "config", "user.name", "Test")
	gitT(t, dir, "config", "user.email", "test@example.com")
	gitT(t, dir, "config", "commit.gpgsign", "false")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

// runIn types a command into a shell session and waits for its marker.
// runIn types cmd into shell session id and waits until it has run. It
// waits for a marker file, not for output: on a busy machine the daemon may
// drop a slow attach stream (by design: it never blocks the PTY), and the
// command's own echo is enough output to trigger that. The stream stays
// open (and drained) until then: closing it right after the input could
// make the daemon drop the input unread.
func runIn(t *testing.T, c *client.Client, id, cmd string) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "done")
	st, err := c.Attach(ctxT(t), id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	go func() {
		for {
			if _, err := st.Recv(); err != nil {
				return
			}
		}
	}()
	if err := st.Input([]byte(cmd + "; touch '" + marker + "'\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(deadline) {
			tail, _ := c.Tail(ctxT(t), id, 12)
			t.Fatalf("command did not finish in session %s: %q\nlast output:\n%s", id, cmd, strings.Join(tail, "\n"))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func listWorktrees(t *testing.T, c *client.Client, projectID string) []protocol.WorktreeInfo {
	t.Helper()
	wts, err := c.Worktrees(ctxT(t), projectID)
	if err != nil {
		t.Fatal(err)
	}
	return wts
}

func TestWorktreeSession(t *testing.T) {
	c, p, stop := startDaemon(t)
	ctx := ctxT(t)
	repo := tempRepo(t)
	proj, err := c.ProjectAdd(ctx, repo, "Mi Repo")
	if err != nil {
		t.Fatal(err)
	}
	pg, err := c.ProjectGit(ctx, proj.ID)
	if err != nil || !pg.IsRepo || pg.CurrentBranch != "main" || pg.DefaultBase != "main" || pg.Error != "" {
		t.Fatalf("project.git = %+v, %v", pg, err)
	}

	// A non-repo project cannot use worktrees.
	plain, _ := c.ProjectAdd(ctx, t.TempDir(), "plain")
	if pg, _ := c.ProjectGit(ctx, plain.ID); pg.IsRepo || pg.Error == "" || pg.ErrorCode != apierr.NotARepo {
		t.Fatalf("project.git(plain) = %+v", pg)
	}
	if _, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: plain.ID, ProfileID: "shell", Mode: protocol.ModeWorktree}); err == nil ||
		!strings.Contains(err.Error(), "not a git repository") || !hasCode(err, apierr.NotARepo) {
		t.Fatalf("worktree spawn in plain dir: %v", err)
	}
	if _, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree, Base: "nope"}); err == nil ||
		!strings.Contains(err.Error(), "does not exist") || !hasCode(err, apierr.BaseMissing) {
		t.Fatalf("missing base: %v", err)
	}

	info, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree})
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(p.RuntimeDir, "worktrees", "mi-repo-"+proj.ID, info.ID)
	if real, err := filepath.EvalSymlinks(p.RuntimeDir); err == nil {
		wantDir = filepath.Join(real, "worktrees", "mi-repo-"+proj.ID, info.ID)
	}
	if info.Mode != protocol.ModeWorktree || info.WorktreeID != info.ID || info.WorktreePath != wantDir || info.Cwd != wantDir ||
		info.Base != "main" || info.Branch != "agent/shell-"+info.ID[:4] {
		t.Fatalf("spawn info = %+v (want dir %s)", info, wantDir)
	}
	if got := gitT(t, wantDir, "branch", "--show-current"); got != info.Branch {
		t.Fatalf("worktree branch = %q", got)
	}

	// Changes made through the session show up in its diff.
	runIn(t, c, info.ID, "pwd > where.txt && git add where.txt && git commit -qm 'add where' && echo two >> a.txt && printf 'x\\000y' > bin.dat")
	d, err := c.SessionDiff(ctx, info.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range d.Files {
		files[f.Path] = f.Status
		if f.Path == "bin.dat" && (!f.Binary || !f.Untracked || f.Patch != "") {
			t.Errorf("bin.dat = %+v", f)
		}
	}
	if len(d.Files) != 3 || files["where.txt"] != "A" || files["a.txt"] != "M" || files["bin.dat"] != "A" {
		t.Fatalf("diff files = %+v", d.Files)
	}
	if len(d.Commits) != 1 || d.Commits[0].Subject != "add where" || d.Git.Ahead != 1 || d.Git.Files != 3 || d.Git.Uncommitted != 2 {
		t.Fatalf("diff commits/git = %+v / %+v", d.Commits, d.Git)
	}
	if !strings.Contains(d.Patch(), "+two") || d.Mode != protocol.ModeWorktree || d.Branch != info.Branch {
		t.Fatalf("patch/meta wrong: %+v\n%s", d, d.Patch())
	}
	// The cached summary reaches session.list.
	if s := findSession(t, c, info.ID); s.Git == nil || s.Git.Files != 3 || s.Git.Ahead != 1 {
		t.Fatalf("session git = %+v", s.Git)
	}
	if wts := listWorktrees(t, c, proj.ID); len(wts) != 1 || !wts[0].Live || wts[0].Orphan || wts[0].Branch != info.Branch {
		t.Fatalf("worktree.list (live) = %+v", wts)
	}
	if _, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: info.ID, Force: true}); err == nil || !strings.Contains(err.Error(), "still running") || !hasCode(err, apierr.WorktreeLive) {
		t.Fatalf("remove with a live session: %v", err)
	}

	// Killing the session keeps the worktree; forgetting it makes it an orphan.
	if _, err := c.Kill(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, c, info.ID, "exited")
	if _, err := os.Stat(filepath.Join(wantDir, "where.txt")); err != nil {
		t.Fatalf("worktree gone after kill: %v", err)
	}
	if wts := listWorktrees(t, c, proj.ID); len(wts) != 1 || wts[0].Live || wts[0].Orphan {
		t.Fatalf("worktree.list (exited) = %+v", wts)
	}
	if _, err := c.Kill(ctx, info.ID); err != nil { // remove from the list
		t.Fatal(err)
	}
	wts := listWorktrees(t, c, proj.ID)
	if len(wts) != 1 || !wts[0].Orphan || wts[0].Git == nil || wts[0].Git.Files != 3 {
		t.Fatalf("worktree.list (orphan) = %+v", wts)
	}
	// An orphan still has a diff.
	if d, err := c.WorktreeDiff(ctx, info.ID, true); err != nil || len(d.Files) != 3 || d.Patch() != "" {
		t.Fatalf("worktree.diff = %+v, %v", d, err)
	}

	// The registry survives a daemon restart.
	stop()
	c, _ = startDaemonIn(t, p, func(daemon.Notification) {})
	ctx = ctxT(t)
	wts = listWorktrees(t, c, proj.ID)
	if len(wts) != 1 || !wts[0].Orphan || wts[0].ID != info.ID {
		t.Fatalf("worktree.list after restart = %+v", wts)
	}

	// Reopen a shell in it.
	re, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", WorktreeID: info.ID})
	if err != nil || re.Cwd != wantDir || re.WorktreeID != info.ID || re.Branch != info.Branch {
		t.Fatalf("reopen = %+v, %v", re, err)
	}
	if wts := listWorktrees(t, c, proj.ID); len(wts) != 1 || !wts[0].Live {
		t.Fatalf("worktree.list (reopened) = %+v", wts)
	}
	if _, err := c.Kill(ctx, re.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, c, re.ID, "exited")

	// Removal refuses to lose uncommitted work unless forced.
	dry, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: info.ID, DeleteBranch: true, DryRun: true})
	if err != nil || dry.Removed || dry.Loss == nil || len(dry.Loss.Uncommitted) != 2 || len(dry.Loss.Unmerged) != 1 {
		t.Fatalf("dry run = %+v, %v", dry, err)
	}
	if _, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: info.ID, DeleteBranch: true}); err == nil ||
		!strings.Contains(err.Error(), "uncommitted") || !strings.Contains(err.Error(), "not merged") || !hasCode(err, apierr.WorktreeDirty) {
		t.Fatalf("unforced remove: %v", err)
	}
	if _, err := os.Stat(wantDir); err != nil {
		t.Fatal("refused remove deleted the worktree")
	}
	res, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: info.ID, DeleteBranch: true, Force: true})
	if err != nil || !res.Removed {
		t.Fatalf("forced remove = %+v, %v", res, err)
	}
	if _, err := os.Stat(wantDir); !os.IsNotExist(err) {
		t.Fatal("worktree folder still there")
	}
	if out := gitT(t, repo, "branch", "--list", info.Branch); out != "" {
		t.Fatalf("branch not deleted: %q", out)
	}
	if wts := listWorktrees(t, c, proj.ID); len(wts) != 0 {
		t.Fatalf("worktree.list after remove = %+v", wts)
	}
}

func TestWorktreeMergeAndSameMode(t *testing.T) {
	c, _, _ := startDaemon(t)
	ctx := ctxT(t)
	repo := tempRepo(t)
	proj, _ := c.ProjectAdd(ctx, repo, "")

	wt, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree, Branch: "Mi Feature"})
	if err != nil || wt.Branch != "Mi-Feature" {
		t.Fatalf("spawn = %+v, %v", wt, err)
	}
	runIn(t, c, wt.ID, "echo feat > feat.txt && git add feat.txt && git commit -qm feat")

	// Refused while the main tree is dirty.
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("dirty\n"), 0o644)
	if _, err := c.WorktreeMerge(ctx, wt.WorktreeID); err == nil || !strings.Contains(err.Error(), "uncommitted") || !hasCode(err, apierr.MergeMainDirty) {
		t.Fatalf("merge into dirty main: %v", err)
	}
	gitT(t, repo, "checkout", "--", "a.txt")
	res, err := c.WorktreeMerge(ctx, wt.WorktreeID)
	if err != nil || res.Commits != 1 || res.Base != "main" {
		t.Fatalf("merge = %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "feat.txt")); err != nil {
		t.Fatal("merged file missing")
	}

	// Hook integration works the same in a worktree: it goes through
	// --settings (a file in the daemon's hooks dir, after the MCP servers'
	// --mcp-config), never project files.
	hk, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "hookedclaude", Mode: protocol.ModeWorktree})
	if err != nil {
		t.Fatal(err)
	}
	if len(hk.Args) < 4 || hk.Args[0] != "--mcp-config" || hk.Args[2] != "--settings" || !filepath.IsAbs(hk.Args[3]) || strings.HasPrefix(hk.Args[3], hk.WorktreePath) {
		t.Fatalf("hook args in worktree = %v (worktree %s)", hk.Args, hk.WorktreePath)
	}
	if _, err := os.Stat(filepath.Join(hk.WorktreePath, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("hook setup wrote into the worktree: %v", err)
	}

	// Same-folder sessions diff the project against HEAD.
	same, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell"})
	if err != nil || same.Mode != protocol.ModeSame || same.Cwd != repo || same.WorktreeID != "" {
		t.Fatalf("same spawn = %+v, %v", same, err)
	}
	runIn(t, c, same.ID, "echo new > untracked.txt")
	// GitInfo is refreshed lazily once the shell goes idle.
	deadline := time.Now().Add(10 * time.Second)
	for {
		s := findSession(t, c, same.ID)
		if s.Git != nil && s.Git.Files == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("same session git = %+v", s.Git)
		}
		time.Sleep(50 * time.Millisecond)
	}
	d, err := c.SessionDiff(ctx, same.ID, false)
	if err != nil || d.Mode != protocol.ModeSame || len(d.Files) != 1 || d.Files[0].Path != "untracked.txt" || !d.Files[0].Untracked || d.Branch != "main" {
		t.Fatalf("same diff = %+v, %v", d, err)
	}
}

func TestWorktreePR(t *testing.T) {
	c, _, _ := startDaemon(t)
	ctx := ctxT(t)
	repo := tempRepo(t)
	proj, _ := c.ProjectAdd(ctx, repo, "")
	wt, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree, Branch: "agent/pr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.WorktreePR(ctx, wt.WorktreeID, false); err == nil || !hasCode(err, apierr.PRNothing) {
		t.Fatalf("pr with nothing ahead: %v", err)
	}
	os.WriteFile(filepath.Join(wt.WorktreePath, "feat.txt"), []byte("feat\n"), 0o644)
	gitT(t, wt.WorktreePath, "add", "feat.txt")
	gitT(t, wt.WorktreePath, "commit", "-qm", "feat")
	if _, err := c.WorktreePR(ctx, wt.WorktreeID, false); err == nil || !hasCode(err, apierr.PRNoRemote) {
		t.Fatalf("pr without remote: %v", err)
	}

	bare := filepath.Join(t.TempDir(), "remote.git")
	gitT(t, repo, "init", "-q", "--bare", "-b", "main", bare)
	gitT(t, repo, "remote", "add", "origin", bare)
	dry, err := c.WorktreePR(ctx, wt.WorktreeID, true)
	if err != nil || dry.Pushed || dry.Remote != "origin" || dry.Commits != 1 || dry.Base != "main" {
		t.Fatalf("dry pr = %+v, %v", dry, err)
	}
	res, err := c.WorktreePR(ctx, wt.WorktreeID, false)
	if err != nil || !res.Pushed || res.Branch != "agent/pr" || res.URL != "" {
		t.Fatalf("pr = %+v, %v", res, err)
	}
	if got, want := gitT(t, bare, "rev-parse", "refs/heads/agent/pr"), gitT(t, wt.WorktreePath, "rev-parse", "HEAD"); got != want {
		t.Fatalf("remote branch at %s, want %s", got, want)
	}
	if out := gitT(t, bare, "for-each-ref", "--format=%(refname)"); out != "refs/heads/agent/pr" {
		t.Fatalf("remote refs = %q (base must not be pushed)", out)
	}
}

// hasCode reports whether a daemon error carries code (protocol errorCode).
func hasCode(err error, code string) bool {
	c, _, ok := apierr.Code(err)
	return ok && c == code
}

// orphanWorktree spawns a shell in a new worktree and forgets the session,
// leaving the worktree orphaned; it returns the worktree's id and folder.
func orphanWorktree(t *testing.T, c *client.Client, projectID string) (string, string) {
	t.Helper()
	ctx := ctxT(t)
	info, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: projectID, ProfileID: "shell", Mode: protocol.ModeWorktree})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Kill(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, c, info.ID, "exited")
	if _, err := c.Kill(ctx, info.ID); err != nil { // remove from the list
		t.Fatal(err)
	}
	return info.WorktreeID, info.WorktreePath
}

func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", msg)
}

func TestWorktreeRemoveMerged(t *testing.T) {
	c, _, _ := startDaemon(t)
	ctx := ctxT(t)
	repo := tempRepo(t)
	proj, err := c.ProjectAdd(ctx, repo, "merged")
	if err != nil {
		t.Fatal(err)
	}
	squashed, sqDir := orphanWorktree(t, c, proj.ID) // squash-merged into main
	merged, mDir := orphanWorktree(t, c, proj.ID)    // merged with a merge commit
	dirty, dDir := orphanWorktree(t, c, proj.ID)     // squash-merged, then edited
	open, oDir := orphanWorktree(t, c, proj.ID)      // a commit main lacks
	fresh, _ := orphanWorktree(t, c, proj.ID)        // no commits at all

	commitFile(t, sqDir, "sq.txt", "sq\n", "sq work")
	commitFile(t, repo, "sq.txt", "sq\n", "squashed sq")
	commitFile(t, mDir, "m.txt", "m\n", "m work")
	mBranch := gitT(t, mDir, "branch", "--show-current")
	gitT(t, repo, "merge", "-q", "--no-ff", "-m", "merge m", mBranch)
	commitFile(t, dDir, "d.txt", "d\n", "d work")
	commitFile(t, repo, "d.txt", "d\n", "squashed d")
	os.WriteFile(filepath.Join(dDir, "d.txt"), []byte("edited after\n"), 0o644)
	commitFile(t, oDir, "o.txt", "o\n", "o work")

	byID := map[string]protocol.WorktreeInfo{}
	for _, w := range listWorktrees(t, c, proj.ID) {
		byID[w.ID] = w
	}
	for id, want := range map[string]bool{squashed: true, merged: true, dirty: true, open: false, fresh: false} {
		if w := byID[id]; w.Git == nil || w.Git.Merged != want {
			t.Errorf("%s: git = %+v, want merged %v", filepath.Base(w.Path), w.Git, want)
		}
	}
	// Removing a squash-merged worktree with its branch needs no force.
	if dry, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: squashed, DeleteBranch: true, DryRun: true}); err != nil || dry.Loss != nil {
		t.Fatalf("dry run on a squash-merged worktree = %+v, %v", dry.Loss, err)
	}

	ids := func(ws []protocol.WorktreeInfo) string {
		var out []string
		for _, w := range ws {
			out = append(out, w.ID)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	want := []string{squashed, merged}
	sort.Strings(want)
	dry, err := c.WorktreeRemoveMerged(ctx, protocol.WorktreeRemoveMergedParams{ProjectID: proj.ID, DryRun: true})
	if err != nil || ids(dry.Removed) != strings.Join(want, ",") || len(dry.Skipped) != 1 || dry.Skipped[0].Worktree.ID != dirty ||
		!strings.Contains(dry.Skipped[0].Reason, "uncommitted") {
		t.Fatalf("dry run = %+v, %v", dry, err)
	}
	if len(listWorktrees(t, c, proj.ID)) != 5 {
		t.Fatal("dry run removed something")
	}

	res, err := c.WorktreeRemoveMerged(ctx, protocol.WorktreeRemoveMergedParams{ProjectID: proj.ID})
	if err != nil || ids(res.Removed) != strings.Join(want, ",") || len(res.Skipped) != 1 {
		t.Fatalf("remove merged = %+v, %v", res, err)
	}
	left := listWorktrees(t, c, proj.ID)
	stay := []string{dirty, open, fresh}
	sort.Strings(stay)
	if got := ids(left); got != strings.Join(stay, ",") {
		t.Fatalf("left = %s", got)
	}
	for _, dir := range []string{sqDir, mDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s still there", dir)
		}
	}
	if out := gitT(t, repo, "branch", "--list", mBranch); out != "" {
		t.Errorf("branch %s not deleted", mBranch)
	}
	if _, err := os.Stat(filepath.Join(dDir, "d.txt")); err != nil {
		t.Error("the dirty worktree lost its file")
	}
}

// A pull request squash-merged on the host while the local main was not
// pulled: the diff already shows the branch as merged (against the
// upstream), and removing it with its branch needs no force either.
func TestWorktreeRemoveSquashedOnHostOnly(t *testing.T) {
	c, _, _ := startDaemon(t)
	ctx := ctxT(t)
	repo := tempRepo(t)
	proj, err := c.ProjectAdd(ctx, repo, "host")
	if err != nil {
		t.Fatal(err)
	}
	id, dir := orphanWorktree(t, c, proj.ID)
	commitFile(t, dir, "h.txt", "h\n", "h work")
	branch := gitT(t, dir, "branch", "--show-current")
	// origin/main gets the squash; the local main stays where it was.
	tree := gitT(t, repo, "rev-parse", branch+"^{tree}")
	squash := gitT(t, repo, "commit-tree", "-p", "main", "-m", "squashed h", tree)
	gitT(t, repo, "update-ref", "refs/remotes/origin/main", squash)

	var info protocol.WorktreeInfo
	for _, w := range listWorktrees(t, c, proj.ID) {
		if w.ID == id {
			info = w
		}
	}
	if info.Git == nil || !info.Git.Merged {
		t.Fatalf("not shown as merged: %+v", info.Git)
	}
	dry, err := c.WorktreeRemove(ctx, protocol.WorktreeRemoveParams{ID: id, DeleteBranch: true, DryRun: true})
	if err != nil || dry.Loss != nil {
		t.Fatalf("dry run = %+v, %v", dry.Loss, err)
	}
	res, err := c.WorktreeRemoveMerged(ctx, protocol.WorktreeRemoveMergedParams{ProjectID: proj.ID})
	if err != nil || len(res.Removed) != 1 || res.Removed[0].ID != id || len(res.Skipped) != 0 {
		t.Fatalf("remove merged = %+v, %v", res, err)
	}
	if out := gitT(t, repo, "branch", "--list", branch); out != "" {
		t.Errorf("branch %s not deleted", branch)
	}
}
