package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/elionaz/panal/internal/apierr"
	"github.com/elionaz/panal/internal/gitx"
	"github.com/elionaz/panal/internal/project"
	"github.com/elionaz/panal/internal/protocol"
	"github.com/elionaz/panal/internal/session"
	"github.com/elionaz/panal/internal/settings"
	"github.com/elionaz/panal/internal/worktree"
)

// Git timeouts: queries that feed the UI lazily, and explicit actions.
const (
	gitQueryTO  = 20 * time.Second
	gitActionTO = 60 * time.Second
	// gitPushTO bounds a push (network) for worktree.pr.
	gitPushTO = 120 * time.Second
	// gitMinInterval throttles automatic GitInfo refreshes per session.
	gitMinInterval = 3 * time.Second
)

// sessGit is what the daemon knows about a session's git context.
type sessGit struct {
	Mode       string // same | worktree
	WorktreeID string
	Dir        string // where git runs (worktree path or the session cwd)
	Branch     string
	Base       string
	BaseCommit string
}

// gitEntry is a session's GitInfo cache and refresh bookkeeping.
type gitEntry struct {
	info    *protocol.GitInfo
	running bool
	again   bool
	last    time.Time
	timer   *time.Timer
}

func (s *Server) worktreeRoot() (string, error) {
	st, _ := settings.Load(s.cfg.Paths.Settings())
	root := s.cfg.Paths.WorktreesDir()
	if d := strings.TrimSpace(st.Git.WorktreeDir); d != "" {
		if rest, ok := strings.CutPrefix(d, "~/"); ok {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			d = filepath.Join(home, rest)
		}
		if !filepath.IsAbs(d) {
			return "", fmt.Errorf("git.worktree_dir must be an absolute path: %q", d)
		}
		root = d
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	return root, nil
}

// baseRef is what a worktree's branch (head) is compared with: its base
// branch, or that branch's upstream when the local copy is behind it (see
// gitx.FreshestBase), or the base commit if the branch is gone.
func baseRef(ctx context.Context, dir, base, baseCommit, head string) string {
	if base != "" && gitx.BranchExists(ctx, dir, base) {
		return gitx.FreshestBase(ctx, dir, base, head)
	}
	if baseCommit != "" {
		return baseCommit
	}
	return base
}

// compareBase is where a worktree's changes start: ref is what its branch
// is compared with (baseRef) and from the merge base with it. When the
// branch's commits are already in ref by content (its pull request was
// squash- or rebase-merged on the host), merged is true and from is HEAD:
// only what is left uncommitted counts as a change.
func compareBase(ctx context.Context, g sessGit) (ref, from string, merged bool, err error) {
	ref = baseRef(ctx, g.Dir, g.Base, g.BaseCommit, "HEAD")
	mb, err := gitx.MergeBase(ctx, g.Dir, ref, "HEAD")
	if err != nil {
		return ref, "", false, err
	}
	if gitx.AlreadyIn(ctx, g.Dir, ref, mb, "HEAD") {
		return ref, "HEAD", true, nil
	}
	// The host merged it, the local base was not pulled and the branch
	// starts from where both still agree (so ref is the stale local base):
	// the upstream holds the merge.
	if up := gitx.Upstream(ctx, g.Dir, g.Base); up != "" && up != ref {
		if mbUp, err := gitx.MergeBase(ctx, g.Dir, up, "HEAD"); err == nil && gitx.AlreadyIn(ctx, g.Dir, up, mbUp, "HEAD") {
			return up, "HEAD", true, nil
		}
	}
	return ref, mb, false, nil
}

// createWorktree makes a new worktree for session id of project proj and
// records it. It returns the record and the session's cwd inside it.
func (s *Server) createWorktree(proj project.Project, profileID, id, branch, base string) (worktree.Record, string, error) {
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	if !gitx.IsRepo(ctx, proj.Path) {
		return worktree.Record{}, "", apierr.New(apierr.NotARepo, map[string]any{"dir": proj.Path, "project": proj.Name},
			"project %s is not a git repository: worktree mode is not available", proj.Name)
	}
	top, err := gitx.RepoRoot(ctx, proj.Path)
	if err != nil {
		return worktree.Record{}, "", err
	}
	repo, err := gitx.MainWorktree(ctx, top)
	if err != nil {
		return worktree.Record{}, "", err
	}
	if base == "" {
		if base, err = gitx.DefaultBase(ctx, proj.Path); err != nil {
			return worktree.Record{}, "", err
		}
	}
	baseCommit, err := gitx.ResolveCommit(ctx, repo, base)
	if err != nil {
		return worktree.Record{}, "", err
	}
	name := gitx.SanitizeBranch(branch)
	if name == "" {
		name = gitx.SanitizeBranch("agent/" + profileID + "-" + id[:min(4, len(id))])
	}
	name = gitx.UniqueBranch(ctx, repo, name)
	root, err := s.worktreeRoot()
	if err != nil {
		return worktree.Record{}, "", err
	}
	path := filepath.Join(root, worktree.ProjectSlug(proj.Name, proj.ID), id)
	if err := gitx.AddWorktree(ctx, repo, path, name, base); err != nil {
		return worktree.Record{}, "", err
	}
	rec := worktree.Record{
		ID: id, ProjectID: proj.ID, Repo: repo, Path: path, Branch: name,
		Base: base, BaseCommit: baseCommit, ProfileID: profileID, CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if rel, err := filepath.Rel(canonPath(top), canonPath(proj.Path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		rec.Subdir = rel
	}
	if err := s.wts.Put(rec); err != nil {
		_ = gitx.RemoveWorktree(ctx, repo, path, name, gitx.RemoveOptions{Force: true, DeleteBranch: true})
		return worktree.Record{}, "", err
	}
	s.log.Info("worktree created", "id", id, "path", path, "branch", name, "base", base)
	return rec, recCwd(rec), nil
}

// recCwd is where a session in rec starts: the project's subdirectory
// inside the worktree (if it still exists), else its root.
func recCwd(rec worktree.Record) string {
	if rec.Subdir != "" {
		d := filepath.Join(rec.Path, rec.Subdir)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return rec.Path
}

// discardWorktree undoes createWorktree (the session failed to start).
func (s *Server) discardWorktree(rec worktree.Record) {
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	if err := gitx.RemoveWorktree(ctx, rec.Repo, rec.Path, rec.Branch, gitx.RemoveOptions{Force: true, DeleteBranch: true}); err != nil {
		s.log.Warn("discard worktree", "id", rec.ID, "err", err)
	}
	_ = s.wts.Delete(rec.ID)
}

func canonPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// sessionGit returns a session's git context (nil if none recorded).
func (s *Server) sessionGit(id string) *sessGit {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.git[id]; ok {
		return &g
	}
	return nil
}

// gitInfoFor computes the change summary of a git context (nil when the
// folder is not a git work tree).
func gitInfoFor(ctx context.Context, g sessGit) (*protocol.GitInfo, error) {
	if _, err := os.Stat(g.Dir); err != nil || !gitx.IsRepo(ctx, g.Dir) {
		return nil, nil
	}
	info := &protocol.GitInfo{At: time.Now().UTC()}
	from := ""
	if g.Mode == protocol.ModeWorktree {
		ref, mb, merged, err := compareBase(ctx, g)
		if err != nil {
			return nil, err
		}
		from = mb
		if !merged {
			if info.Ahead, err = gitx.AheadCount(ctx, g.Dir, ref, "HEAD"); err != nil {
				return nil, err
			}
		}
		info.Merged = merged || (info.Ahead == 0 && branchMoved(ctx, g))
	}
	n, err := gitx.ChangedFiles(ctx, g.Dir, from)
	if err != nil {
		return nil, err
	}
	info.Files = n
	st, err := gitx.GetStatus(ctx, g.Dir)
	if err != nil {
		return nil, err
	}
	info.Uncommitted = st.Changed()
	info.Head, _ = gitx.CurrentBranch(ctx, g.Dir)
	return info, nil
}

// requestGit schedules a throttled GitInfo refresh of a session: at most
// one git run at a time per session and one every gitMinInterval; calls
// in between coalesce into a single follow-up run.
func (s *Server) requestGit(id string) {
	if s.sessionGit(id) == nil {
		return
	}
	s.gitMu.Lock()
	defer s.gitMu.Unlock()
	if s.bg.Err() != nil {
		return
	}
	e := s.gitCache[id]
	if e == nil {
		e = &gitEntry{}
		s.gitCache[id] = e
	}
	if e.running {
		e.again = true
		return
	}
	if e.timer != nil {
		return // a run is already scheduled
	}
	wait := gitMinInterval - time.Since(e.last)
	if wait <= 0 {
		e.running = true
		s.bgWG.Go(func() { s.refreshGit(id) })
		return
	}
	e.timer = time.AfterFunc(wait, func() {
		s.gitMu.Lock()
		e.timer = nil
		if s.bg.Err() != nil || e.running {
			e.again = e.running
			s.gitMu.Unlock()
			return
		}
		e.running = true
		s.bgWG.Add(1)
		s.gitMu.Unlock()
		defer s.bgWG.Done()
		s.refreshGit(id)
	})
}

func (s *Server) refreshGit(id string) {
	info, err := s.computeGit(id)
	s.gitMu.Lock()
	e := s.gitCache[id]
	if e == nil { // session forgotten meanwhile
		s.gitMu.Unlock()
		return
	}
	e.running = false
	e.last = time.Now()
	again := e.again
	e.again = false
	s.gitMu.Unlock()
	if err != nil {
		s.log.Debug("git refresh", "id", id, "err", err)
	} else {
		s.storeGit(id, info)
	}
	if again {
		s.requestGit(id)
	}
}

func (s *Server) computeGit(id string) (*protocol.GitInfo, error) {
	g := s.sessionGit(id)
	if g == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(s.bg, gitQueryTO)
	defer cancel()
	s.maybeFetch(*g)
	return gitInfoFor(ctx, *g)
}

// gitFetchInterval is how often, at most, the daemon fetches a worktree
// base's upstream (per repository and branch), so that pull requests
// merged on the host stop counting as changes without a manual pull.
const gitFetchInterval = 5 * time.Minute

// maybeFetch fetches, in the background and at most every
// gitFetchInterval, the upstream of a worktree's base branch; when that
// moved, the sessions of the repository refresh their git info. A fetch
// that fails (offline, no credential: it never prompts) waits for the
// next interval like one that worked.
func (s *Server) maybeFetch(g sessGit) {
	s.fetchBase(g, gitFetchInterval, "")
}

// mergedFetchInterval is how often, at most, forge.status fetches a
// worktree base's upstream when the host says the branch's pull request
// was merged and the local copy does not show it yet: Changes drops to
// zero right away instead of after the next periodic fetch.
const mergedFetchInterval = 30 * time.Second

// fetchBase fetches, in the background, the upstream of worktree g's base
// branch, unless the last fetch of it for the same reason (slot) was less
// than every ago; when it moved, the sessions of the repository refresh
// their git info. Each slot has its own clock: a periodic fetch just
// before a merge does not hold back the one the merge asks for.
func (s *Server) fetchBase(g sessGit, every time.Duration, slot string) {
	if g.Mode != protocol.ModeWorktree || g.Base == "" || g.WorktreeID == "" || every <= 0 {
		return
	}
	rec, err := s.wts.Get(g.WorktreeID)
	if err != nil {
		return
	}
	key := rec.Repo + "\x00" + g.Base + "\x00" + slot
	s.gitMu.Lock()
	if s.bg.Err() != nil || time.Since(s.fetched[key]) < every {
		s.gitMu.Unlock()
		return
	}
	s.fetched[key] = time.Now()
	s.bgWG.Add(1)
	s.gitMu.Unlock()
	go func() {
		defer s.bgWG.Done()
		ctx, cancel := context.WithTimeout(s.bg, gitQueryTO)
		defer cancel()
		up := gitx.Upstream(ctx, rec.Repo, g.Base)
		remote, branch, ok := gitx.SplitRemoteRef(up)
		if !ok {
			return
		}
		before, _ := gitx.ResolveCommit(ctx, rec.Repo, up)
		if err := gitx.Fetch(ctx, rec.Repo, remote, branch); err != nil {
			s.log.Debug("git fetch", "repo", rec.Repo, "remote", remote, "branch", branch, "err", err)
			return
		}
		if after, _ := gitx.ResolveCommit(ctx, rec.Repo, up); after == before {
			return
		}
		s.log.Info("base upstream moved", "repo", rec.Repo, "ref", gitx.ShortRef(up))
		for _, id := range s.sessionsOfRepo(rec.Repo) {
			s.requestGit(id)
		}
	}()
}

// sessionsOfRepo lists the sessions in worktrees of repository repo.
func (s *Server) sessionsOfRepo(repo string) []string {
	s.mu.Lock()
	ids := make([]string, 0, len(s.git))
	wts := make([]string, 0, len(s.git))
	for id, g := range s.git {
		if g.WorktreeID != "" {
			ids = append(ids, id)
			wts = append(wts, g.WorktreeID)
		}
	}
	s.mu.Unlock()
	var out []string
	for i, id := range ids {
		if rec, err := s.wts.Get(wts[i]); err == nil && rec.Repo == repo {
			out = append(out, id)
		}
	}
	return out
}

// storeGit caches info and publishes session.git if it changed.
func (s *Server) storeGit(id string, info *protocol.GitInfo) {
	s.gitMu.Lock()
	e := s.gitCache[id]
	if e == nil {
		s.gitMu.Unlock()
		return
	}
	changed := !sameGit(e.info, info)
	e.info = info
	s.gitMu.Unlock()
	if info != nil && info.Head != "" {
		s.noteBranch(id, info.Head)
	}
	if changed {
		s.bus.publish(protocol.Event{Type: protocol.EventSessionGit, SessionID: id, At: time.Now().UTC(), Git: info})
	}
}

func sameGit(a, b *protocol.GitInfo) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Files == b.Files && a.Uncommitted == b.Uncommitted && a.Ahead == b.Ahead && a.Head == b.Head
}

func (s *Server) cachedGit(id string) *protocol.GitInfo {
	s.gitMu.Lock()
	defer s.gitMu.Unlock()
	if e := s.gitCache[id]; e != nil && e.info != nil {
		c := *e.info
		return &c
	}
	return nil
}

// forgetGit drops a removed session's git state.
func (s *Server) forgetGit(id string) {
	s.gitMu.Lock()
	if e := s.gitCache[id]; e != nil && e.timer != nil {
		e.timer.Stop()
	}
	delete(s.gitCache, id)
	s.gitMu.Unlock()
	s.mu.Lock()
	delete(s.git, id)
	s.mu.Unlock()
}

// stopGit cancels background git work and waits for it (daemon shutdown).
func (s *Server) stopGit() {
	s.gitMu.Lock()
	s.bgCancel()
	for _, e := range s.gitCache {
		if e.timer != nil {
			e.timer.Stop()
		}
	}
	s.gitMu.Unlock()
	s.bgWG.Wait()
}

func (s *Server) publishWorktrees(projectID string) {
	s.bus.publish(protocol.Event{Type: protocol.EventWorktrees, ProjectID: projectID, At: time.Now().UTC()})
}

// --- RPCs ----------------------------------------------------------------

func (s *Server) projectGit(p protocol.ProjectGitParams) (protocol.ProjectGit, error) {
	proj, err := s.projects.Get(p.ProjectID)
	if err != nil {
		return protocol.ProjectGit{}, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitQueryTO)
	defer cancel()
	out := protocol.ProjectGit{Branches: []string{}}
	if !gitx.Available() {
		out.Error, out.ErrorCode = "git is not installed", apierr.GitMissing
		return out, nil
	}
	if !gitx.IsRepo(ctx, proj.Path) {
		out.Error, out.ErrorCode = "the project is not a git repository", apierr.NotARepo
		return out, nil
	}
	out.IsRepo = true
	out.Root, _ = gitx.RepoRoot(ctx, proj.Path)
	out.CurrentBranch, _ = gitx.CurrentBranch(ctx, proj.Path)
	out.DefaultBase, _ = gitx.DefaultBase(ctx, proj.Path)
	if bs, err := gitx.Branches(ctx, proj.Path); err == nil && bs != nil {
		out.Branches = bs
	}
	if !gitx.HasCommits(ctx, proj.Path) {
		out.Error, out.ErrorCode = "the repository has no commits yet", apierr.NoCommits
	}
	return out, nil
}

// diff serves session.diff and worktree.diff.
func (s *Server) diff(p protocol.DiffParams) (protocol.DiffResult, error) {
	var g sessGit
	switch {
	case p.SessionID != "":
		sg := s.sessionGit(p.SessionID)
		if sg == nil {
			if _, err := s.lookup(p.SessionID); err != nil {
				return protocol.DiffResult{}, err
			}
			return protocol.DiffResult{}, errors.New("the session has no git information")
		}
		g = *sg
	case p.WorktreeID != "":
		rec, err := s.wts.Get(p.WorktreeID)
		if err != nil {
			return protocol.DiffResult{}, err
		}
		g = recGit(rec)
	default:
		return protocol.DiffResult{}, errors.New("missing sessionId or worktreeId")
	}
	if _, err := os.Stat(g.Dir); err != nil {
		return protocol.DiffResult{}, apierr.New(apierr.DirMissing, map[string]any{"dir": g.Dir}, "the folder no longer exists: %s", g.Dir)
	}
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	if !gitx.IsRepo(ctx, g.Dir) {
		return protocol.DiffResult{}, apierr.New(apierr.NotARepo, map[string]any{"dir": g.Dir}, "%s is not a git repository", g.Dir)
	}
	res := protocol.DiffResult{Mode: g.Mode, Dir: g.Dir, WorktreeID: g.WorktreeID, Branch: g.Branch, Base: g.Base, Commits: []gitx.Commit{}}
	if g.Mode == protocol.ModeWorktree {
		if cur, err := gitx.CurrentBranch(ctx, g.Dir); err == nil && cur != "" && cur != g.Branch {
			res.Branch, res.StartBranch = cur, g.Branch
		}
		if rec, err := s.wts.Get(g.WorktreeID); err == nil {
			if bs := worktreeBranches(ctx, rec); len(bs) > 1 {
				res.Branches = bs
			}
		}
	}
	opts := gitx.DiffOptions{StatOnly: p.Stat}
	if g.Mode == protocol.ModeWorktree {
		ref, mb, merged, err := compareBase(ctx, g)
		if err != nil {
			return res, err
		}
		opts.From = mb
		if !merged {
			cs, err := gitx.CommitsAhead(ctx, g.Dir, ref, "HEAD")
			if err != nil {
				return res, err
			}
			if cs != nil {
				res.Commits = cs
			}
			res.Git.Ahead, _ = gitx.AheadCount(ctx, g.Dir, ref, "HEAD")
		}
	} else if b, err := gitx.CurrentBranch(ctx, g.Dir); err == nil {
		res.Branch = b
	}
	d, err := gitx.WorkingDiff(ctx, g.Dir, opts)
	if err != nil {
		return res, err
	}
	res.From, res.Files, res.Additions, res.Deletions, res.Truncated, res.PatchSize =
		d.From, d.Files, d.Additions, d.Deletions, d.Truncated, d.PatchSize
	res.Git.Files = len(d.Files)
	if st, err := gitx.GetStatus(ctx, g.Dir); err == nil {
		res.Git.Uncommitted = st.Changed()
	}
	res.Git.At = time.Now().UTC()
	// The diff is fresh: update the sessions' summaries for free.
	info := res.Git
	for _, id := range s.sessionsIn(g) {
		s.ensureGitEntry(id)
		s.storeGit(id, &info)
	}
	return res, nil
}

func (s *Server) ensureGitEntry(id string) {
	s.gitMu.Lock()
	if s.gitCache[id] == nil {
		s.gitCache[id] = &gitEntry{}
	}
	s.gitMu.Unlock()
}

// sessionsIn lists the sessions whose git context is g's folder.
func (s *Server) sessionsIn(g sessGit) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, sg := range s.git {
		if sg.Mode == g.Mode && (sg.WorktreeID != "" && sg.WorktreeID == g.WorktreeID || sg.WorktreeID == "" && sg.Dir == g.Dir) {
			ids = append(ids, id)
		}
	}
	return ids
}

func recGit(rec worktree.Record) sessGit {
	return sessGit{Mode: protocol.ModeWorktree, WorktreeID: rec.ID, Dir: rec.Path, Branch: rec.Branch, Base: rec.Base, BaseCommit: rec.BaseCommit}
}

// worktreeList lists a project's worktrees. Records whose folder is gone
// are dropped (and pruned); panal worktrees git knows about but the
// registry does not (lost worktrees.json) are adopted.
func (s *Server) worktreeList(p protocol.WorktreeListParams) (protocol.WorktreeList, error) {
	proj, err := s.projects.Get(p.ProjectID)
	if err != nil {
		return protocol.WorktreeList{}, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitQueryTO)
	defer cancel()
	s.reconcile(ctx, proj)

	// Sessions per worktree.
	type use struct {
		ids  []string
		live bool
	}
	uses := map[string]*use{}
	var check []*session.Session // whose liveness decides u.live
	var checkWT []string
	s.mu.Lock()
	for _, id := range s.order {
		g, ok := s.git[id]
		if !ok || g.WorktreeID == "" {
			continue
		}
		u := uses[g.WorktreeID]
		if u == nil {
			u = &use{}
			uses[g.WorktreeID] = u
		}
		u.ids = append(u.ids, id)
		if sess := s.sessions[id]; sess != nil {
			check, checkWT = append(check, sess), append(checkWT, g.WorktreeID)
		}
	}
	s.mu.Unlock()
	// Alive takes the session's status lock, whose change callbacks take
	// s.mu (publishState): never call it with s.mu held.
	for i, sess := range check {
		if sess.Alive() {
			uses[checkWT[i]].live = true
		}
	}

	out := protocol.WorktreeList{Worktrees: []protocol.WorktreeInfo{}}
	for _, rec := range s.wts.List(proj.ID) {
		wi := protocol.WorktreeInfo{
			ID: rec.ID, ProjectID: rec.ProjectID, Path: rec.Path, Branch: rec.Branch, Base: rec.Base,
			ProfileID: rec.ProfileID, CreatedAt: rec.CreatedAt, SessionIDs: []string{},
		}
		if u := uses[rec.ID]; u != nil {
			wi.SessionIDs, wi.Live = u.ids, u.live
		}
		wi.Orphan = len(wi.SessionIDs) == 0
		if wi.Orphan {
			if info, err := gitInfoFor(ctx, recGit(rec)); err == nil {
				wi.Git = info
			}
		} else {
			wi.Git = s.cachedGit(wi.SessionIDs[len(wi.SessionIDs)-1])
		}
		out.Worktrees = append(out.Worktrees, wi)
	}
	return out, nil
}

// reconcile syncs the registry with git for one project.
func (s *Server) reconcile(ctx context.Context, proj project.Project) {
	changed := false
	repos := map[string]bool{}
	for _, rec := range s.wts.List(proj.ID) {
		repos[rec.Repo] = true
		if _, err := os.Stat(rec.Path); errors.Is(err, os.ErrNotExist) {
			s.log.Info("worktree folder gone: forgetting it", "id", rec.ID, "path", rec.Path)
			_ = gitx.Prune(ctx, rec.Repo)
			_ = s.wts.Delete(rec.ID)
			changed = true
		}
	}
	root, err := s.worktreeRoot()
	if err != nil || !gitx.IsRepo(ctx, proj.Path) {
		return
	}
	repo, err := gitx.MainWorktree(ctx, proj.Path)
	if err != nil {
		return
	}
	wts, err := gitx.ListWorktrees(ctx, repo)
	if err != nil {
		return
	}
	for _, w := range wts {
		if w.Prunable || !projectWorktreeDir(canonPath(root), canonPath(filepath.Dir(w.Path)), proj.ID) {
			continue
		}
		id := filepath.Base(w.Path)
		if _, err := s.wts.Get(id); err == nil {
			continue
		}
		base, _ := gitx.DefaultBase(ctx, repo)
		rec := worktree.Record{ID: id, ProjectID: proj.ID, Repo: repo, Path: canonPath(w.Path), Branch: w.Branch,
			Base: base, CreatedAt: time.Now().UTC().Truncate(time.Second), Adopted: true}
		if err := s.wts.Put(rec); err == nil {
			s.log.Info("worktree adopted", "id", id, "path", rec.Path)
			changed = true
		}
	}
	if changed {
		s.publishWorktrees(proj.ID)
	}
}

// projectWorktreeDir reports whether dir is one of project id's folders
// under the worktrees root: ProjectSlug(name, id) for whatever name the
// project had when the worktree was made (it may have been renamed since).
func projectWorktreeDir(root, dir, id string) bool {
	if filepath.Dir(dir) != root {
		return false
	}
	base := filepath.Base(dir)
	return base == id || strings.HasSuffix(base, "-"+id)
}

// liveSessionIn returns a running session using worktree id ("" if none).
func (s *Server) liveSessionIn(id string) string {
	s.mu.Lock()
	var in []*session.Session
	for sid, g := range s.git {
		if g.WorktreeID == id {
			if sess := s.sessions[sid]; sess != nil {
				in = append(in, sess)
			}
		}
	}
	s.mu.Unlock()
	// Outside s.mu: see worktreeList.
	for _, sess := range in {
		if sess.Alive() {
			return sess.ID
		}
	}
	return ""
}

func (s *Server) worktreeRemove(p protocol.WorktreeRemoveParams) (protocol.WorktreeRemoveResult, error) {
	rec, err := s.wts.Get(p.ID)
	if err != nil {
		return protocol.WorktreeRemoveResult{}, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	opts := removeOptions(ctx, rec, p.Force, p.DeleteBranch)
	if p.DryRun {
		loss, err := gitx.Losses(ctx, rec.Repo, rec.Path, rec.Branch, opts)
		return protocol.WorktreeRemoveResult{Loss: loss}, err
	}
	if err := s.removeOne(ctx, rec, opts); err != nil {
		var le *gitx.LossError
		if errors.As(err, &le) {
			return protocol.WorktreeRemoveResult{Loss: le}, err
		}
		return protocol.WorktreeRemoveResult{}, err
	}
	s.publishWorktrees(rec.ProjectID)
	return protocol.WorktreeRemoveResult{Removed: true}, nil
}

// removeOptions are the options to remove rec's worktree: its branch's
// commits are checked against the base and, like compareBase does, against
// the base's upstream (a pull request merged on the host, not pulled yet).
func removeOptions(ctx context.Context, rec worktree.Record, force, deleteBranch bool) gitx.RemoveOptions {
	return gitx.RemoveOptions{
		Force:        force,
		DeleteBranch: deleteBranch,
		Base:         baseRef(ctx, rec.Repo, rec.Base, rec.BaseCommit, "refs/heads/"+rec.Branch),
		Upstream:     gitx.Upstream(ctx, rec.Repo, rec.Base),
	}
}

// removeOne removes a worktree no running session uses (with its branch
// when opts say so) and forgets it. The caller publishes the new list.
func (s *Server) removeOne(ctx context.Context, rec worktree.Record, opts gitx.RemoveOptions) error {
	if sid := s.liveSessionIn(rec.ID); sid != "" {
		return apierr.New(apierr.WorktreeLive, map[string]any{"sessionId": sid},
			"session %s is still running in this worktree: stop it before removing the worktree", sid)
	}
	if err := gitx.RemoveWorktree(ctx, rec.Repo, rec.Path, rec.Branch, opts); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(rec.Path)) // the project's folder, if now empty
	if err := s.wts.Delete(rec.ID); err != nil {
		return err
	}
	s.log.Info("worktree removed", "id", rec.ID, "path", rec.Path, "force", opts.Force, "deleteBranch", opts.DeleteBranch)
	for _, id := range s.sessionsIn(recGit(rec)) {
		s.storeGit(id, nil)
	}
	return nil
}

// worktreeRemoveMerged removes a project's orphaned worktrees whose branch
// is already merged and that have nothing uncommitted, with their
// branches. It never forces: one that would lose something (a commit or
// an edit made since the list was read) is skipped and reported.
func (s *Server) worktreeRemoveMerged(p protocol.WorktreeRemoveMergedParams) (protocol.WorktreeRemoveMergedResult, error) {
	out := protocol.WorktreeRemoveMergedResult{Removed: []protocol.WorktreeInfo{}, Skipped: []protocol.WorktreeSkipped{}}
	list, err := s.worktreeList(protocol.WorktreeListParams{ProjectID: p.ProjectID})
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	removed := false
	for _, wi := range list.Worktrees {
		if !wi.Orphan || wi.Git == nil || !wi.Git.Merged {
			continue
		}
		if wi.Git.Uncommitted > 0 {
			out.Skipped = append(out.Skipped, protocol.WorktreeSkipped{Worktree: wi, Reason: fmt.Sprintf("%d uncommitted %s", wi.Git.Uncommitted, plural(wi.Git.Uncommitted, "change", "changes"))})
			continue
		}
		rec, err := s.wts.Get(wi.ID)
		if err != nil {
			continue
		}
		opts := removeOptions(ctx, rec, false, true)
		if p.DryRun {
			if loss, err := gitx.Losses(ctx, rec.Repo, rec.Path, rec.Branch, opts); err != nil || loss != nil {
				out.Skipped = append(out.Skipped, protocol.WorktreeSkipped{Worktree: wi, Reason: skipReason(err, loss)})
				continue
			}
			out.Removed = append(out.Removed, wi)
			continue
		}
		if err := s.removeOne(ctx, rec, opts); err != nil {
			out.Skipped = append(out.Skipped, protocol.WorktreeSkipped{Worktree: wi, Reason: skipReason(err, nil)})
			continue
		}
		out.Removed = append(out.Removed, wi)
		removed = true
	}
	if removed {
		s.publishWorktrees(p.ProjectID)
	}
	return out, nil
}

func skipReason(err error, loss *gitx.LossError) string {
	if loss != nil {
		return loss.Error()
	}
	return err.Error()
}

func (s *Server) worktreeMerge(p protocol.WorktreeRef) (gitx.MergeResult, error) {
	rec, err := s.wts.Get(p.ID)
	if err != nil {
		return gitx.MergeResult{}, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitActionTO)
	defer cancel()
	branch, err := s.worktreeBranch(ctx, rec)
	if err != nil {
		return gitx.MergeResult{}, err
	}
	res, err := gitx.Merge(ctx, rec.Repo, branch, rec.Base, rec.Path)
	if err != nil {
		return res, err
	}
	s.log.Info("worktree merged", "id", rec.ID, "branch", rec.Branch, "base", rec.Base, "commits", res.Commits)
	for _, id := range s.sessionsIn(recGit(rec)) {
		s.requestGit(id)
	}
	s.publishWorktrees(rec.ProjectID)
	return res, nil
}

// worktreePR pushes a worktree's branch to its remote (see gitx.PullRequest).
func (s *Server) worktreePR(p protocol.WorktreePRParams) (gitx.PRResult, error) {
	rec, err := s.wts.Get(p.ID)
	if err != nil {
		return gitx.PRResult{}, err
	}
	ctx, cancel := context.WithTimeout(s.bg, gitPushTO)
	defer cancel()
	branch, err := s.worktreeBranch(ctx, rec)
	if err != nil {
		return gitx.PRResult{}, err
	}
	res, err := gitx.PullRequest(ctx, rec.Repo, branch, rec.Base, baseRef(ctx, rec.Repo, rec.Base, rec.BaseCommit, "refs/heads/"+branch), rec.Path, p.DryRun)
	if err != nil || p.DryRun {
		return res, err
	}
	s.log.Info("worktree pushed", "id", rec.ID, "branch", branch, "remote", res.Remote, "commits", res.Commits)
	s.addBranch(rec.ID, branch)
	for _, id := range s.sessionsIn(recGit(rec)) {
		s.requestGit(id)
	}
	return res, nil
}

func (s *Server) worktreeOpen(p protocol.WorktreeOpenParams) (map[string]any, error) {
	var dir string
	switch {
	case p.ID != "":
		rec, err := s.wts.Get(p.ID)
		if err != nil {
			return nil, err
		}
		dir = rec.Path
	case p.SessionID != "":
		sess, err := s.lookup(p.SessionID)
		if err != nil {
			return nil, err
		}
		dir = sess.Cwd
		if g := s.sessionGit(p.SessionID); g != nil && g.Dir != "" {
			dir = g.Dir
		}
	default:
		return nil, errors.New("missing id or sessionId")
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, apierr.New(apierr.DirMissing, map[string]any{"dir": dir}, "the folder no longer exists: %s", dir)
	}
	st, _ := settings.Load(s.cfg.Paths.Settings())
	path := s.detector.Path()
	argv, err := gitx.OpenArgv(p.Target, st.Git.Editor, dir, func(cmd string) (string, error) {
		if strings.Contains(cmd, "/") {
			if fi, err := os.Stat(cmd); err == nil && !fi.IsDir() {
				return cmd, nil
			}
			return "", os.ErrNotExist
		}
		for _, d := range filepath.SplitList(path) {
			c := filepath.Join(d, cmd)
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
				return c, nil
			}
		}
		return "", os.ErrNotExist
	})
	if err != nil {
		return nil, err
	}
	env := append(os.Environ(), "PATH="+path)
	if err := gitx.Launch(s.bg, argv, env); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "argv": argv}, nil
}

// gitState groups the daemon's git bookkeeping (embedded in Server).
type gitState struct {
	wts      *worktree.Store
	git      map[string]sessGit // by session id; guarded by Server.mu
	gitMu    sync.Mutex
	gitCache map[string]*gitEntry
	fetched  map[string]time.Time // last fetch per repo + base; guarded by gitMu
	bg       context.Context
	bgCancel context.CancelFunc
	bgWG     sync.WaitGroup
}

// worktreeBranch is the branch worktree rec is on now (its sessions may
// have switched branches in it), falling back to the one it was created
// on for a detached HEAD. Being on the base itself is refused: there is
// nothing of the worktree's own to merge or push.
func (s *Server) worktreeBranch(ctx context.Context, rec worktree.Record) (string, error) {
	cur, err := gitx.CurrentBranch(ctx, rec.Path)
	if err != nil || cur == "" {
		return rec.Branch, nil
	}
	if cur == rec.Base || cur == strings.TrimPrefix(rec.Base, "origin/") {
		return "", apierr.New(apierr.ForgeOnBase, map[string]any{"branch": cur},
			"the worktree is on %s, its base branch: switch it to a branch of its own first", cur)
	}
	return cur, nil
}

// noteBranch records that session id's worktree is on branch (see
// worktree.Record.Branches); the base itself is not one of its branches.
func (s *Server) noteBranch(id, branch string) {
	g := s.sessionGit(id)
	if g == nil || g.Mode != protocol.ModeWorktree || g.WorktreeID == "" {
		return
	}
	if branch == g.Base || branch == strings.TrimPrefix(g.Base, "origin/") {
		return
	}
	s.addBranch(g.WorktreeID, branch)
}

// addBranch adds branch to worktree id's history and drops its cached
// pull request list when that changed it.
func (s *Server) addBranch(worktreeID, branch string) {
	changed, err := s.wts.AddBranch(worktreeID, branch)
	if err != nil {
		s.log.Debug("worktree branch", "id", worktreeID, "err", err)
		return
	}
	if changed {
		s.forgeListForget(worktreeID)
	}
}

// worktreeBranches is every branch worktree rec worked on, newest first:
// what its HEAD reflog shows it checked out (so several switches within
// one turn all count) and what was recorded (pushes, the creation branch).
// Its base is not one of them.
func worktreeBranches(ctx context.Context, rec worktree.Record) []string {
	base := strings.TrimPrefix(rec.Base, "origin/")
	var out []string
	seen := map[string]bool{rec.Base: true, base: true}
	for _, b := range append(gitx.VisitedBranches(ctx, rec.Path), rec.AllBranches()...) {
		if b != "" && !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	return out
}

// branchMoved reports whether a worktree's HEAD has commits beyond the one
// it was created from: with nothing ahead of the base, that means they were
// merged (a fresh worktree has none and is not "merged").
func branchMoved(ctx context.Context, g sessGit) bool {
	if g.BaseCommit == "" {
		return false
	}
	head, err := gitx.ResolveCommit(ctx, g.Dir, "HEAD")
	return err == nil && head != g.BaseCommit && gitx.IsAncestor(ctx, g.Dir, g.BaseCommit, head)
}
