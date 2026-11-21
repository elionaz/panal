package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/elionaz/panal/internal/apierr"
	"github.com/elionaz/panal/internal/forge"
	"github.com/elionaz/panal/internal/protocol"
)

// forge.list: the pull requests of every branch a session's worktree
// worked on (worktree.Record.Branches plus the one it is on), so a session
// that opened several shows them all. A forge.Lister answers for all of
// them in one go (GitHub: two listings); otherwise, or for the branches it
// could not rule out, one host call per branch (at most forgeListParallel
// at a time). Answers are cached forgeListTTL per
// worktree and dropped when its branches or pull requests change.

const (
	forgeListTTL      = time.Minute
	forgeListParallel = 4
	forgeListCallTO   = 20 * time.Second
)

type forgeListEntry struct {
	at   time.Time
	list protocol.ForgeList
}

type forgeListCache struct {
	mu sync.Mutex
	m  map[string]forgeListEntry // by worktree id (or "s:"+session id)
}

func (s *Server) forgeListForget(worktreeID string) {
	s.flist.mu.Lock()
	delete(s.flist.m, worktreeID)
	s.flist.mu.Unlock()
}

func (s *Server) forgeList(p protocol.ForgeListParams) (protocol.ForgeList, error) {
	ref := p.ForgeRef
	ref.Branch = ""
	key := ref.WorktreeID
	if key == "" {
		if g := s.sessionGit(ref.SessionID); g != nil && g.WorktreeID != "" {
			key = g.WorktreeID
		} else {
			key = "s:" + ref.SessionID
		}
	}
	if !p.Refresh {
		s.flist.mu.Lock()
		e, ok := s.flist.m[key]
		s.flist.mu.Unlock()
		if ok && time.Since(e.at) < forgeListTTL {
			return e.list, nil
		}
	}
	ctx, cancel := context.WithTimeout(s.bg, gitPushTO)
	defer cancel()

	out := protocol.ForgeList{PRs: []protocol.ForgeListItem{}}
	t, err := s.forgeTarget(ctx, ref)
	if err != nil && codeOf(err) == apierr.ForgeOnBase && t.dir != "" {
		// On the base now: its other branches may still have pull requests.
		err = nil
		t.current = ""
	}
	if err != nil {
		if unavailableCodes[codeOf(err)] {
			out.Unavailable = codeOf(err)
			return out, nil
		}
		return out, err
	}
	branches := []string{}
	if t.current != "" {
		branches = append(branches, t.current)
	}
	if t.g.Mode == protocol.ModeWorktree {
		if rec, err := s.wts.Get(t.g.WorktreeID); err == nil {
			for _, b := range worktreeBranches(ctx, rec) {
				if b != t.current {
					branches = append(branches, b)
				}
			}
		}
	}
	if t.remoteURL == "" {
		return out, nil // forgeTarget stopped before the remote (on the base)
	}
	f, err := s.forgeFor(t)
	if err != nil {
		if unavailableCodes[codeOf(err)] {
			out.Unavailable = codeOf(err)
			return out, nil
		}
		return out, err
	}
	items := make([]protocol.ForgeListItem, len(branches))
	// One or two host calls for all the branches when the forge can; per
	// branch only for those it could not rule out.
	ask := branches
	if l, ok := f.(forge.Lister); ok {
		cctx, cancel := context.WithTimeout(ctx, forgeListCallTO)
		prs, complete, err := l.Statuses(cctx, branches)
		cancel()
		if err == nil {
			ask = nil
			for i, b := range branches {
				if pr := prs[b]; pr != nil {
					items[i] = protocol.ForgeListItem{Branch: b, PR: pr, Current: b == t.current}
				} else if !complete {
					ask = append(ask, b)
				}
			}
		} else {
			s.log.Debug("forge.list", "statuses", err)
		}
	}
	idx := make(map[string]int, len(branches))
	for i, b := range branches {
		idx[b] = i
	}
	sem := make(chan struct{}, forgeListParallel)
	var wg sync.WaitGroup
	for _, b := range ask {
		i := idx[b]
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, forgeListCallTO)
			defer cancel()
			pr, err := f.Status(cctx, b)
			if err != nil {
				s.log.Debug("forge.list", "branch", b, "err", err)
				return
			}
			items[i] = protocol.ForgeListItem{Branch: b, PR: pr, Current: b == t.current}
		})
	}
	wg.Wait()
	for _, it := range items {
		if it.PR != nil {
			out.PRs = append(out.PRs, it)
		}
	}
	s.flist.mu.Lock()
	if s.flist.m == nil {
		s.flist.m = map[string]forgeListEntry{}
	}
	s.flist.m[key] = forgeListEntry{at: time.Now(), list: out}
	s.flist.mu.Unlock()
	return out, nil
}

func codeOf(err error) string {
	c, _, _ := apierr.Code(err)
	return c
}
