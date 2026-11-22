package forge

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// github drives gh. Every command names the repository (-R owner/repo) so
// gh never has to guess it from the remotes (it asks when there are
// several, and there is no one to answer).
type github struct{ *cli }

func (g *github) Kind() string { return GitHub }
func (g *github) Tool() string { return "gh" }
func (g *github) repo() string { return g.remote.Path }

const ghPRFields = "number,title,state,isDraft,url,baseRefName,headRefName,isCrossRepository,reviewDecision,mergeable,mergeStateStatus,statusCheckRollup"

type ghPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"` // OPEN | CLOSED | MERGED
	IsDraft     bool   `json:"isDraft"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	// IsCrossRepository: the head is a fork's branch (same name, someone
	// else's pull request).
	IsCrossRepository bool   `json:"isCrossRepository"`
	ReviewDecision    string `json:"reviewDecision"`   // APPROVED | CHANGES_REQUESTED | REVIEW_REQUIRED | ""
	Mergeable         string `json:"mergeable"`        // MERGEABLE | CONFLICTING | UNKNOWN
	MergeStateStatus  string `json:"mergeStateStatus"` // CLEAN | BLOCKED | BEHIND | DIRTY | UNSTABLE | …
	StatusCheckRollup []struct {
		Typename   string `json:"__typename"` // CheckRun | StatusContext
		Name       string `json:"name"`
		Context    string `json:"context"`
		Status     string `json:"status"`     // CheckRun: QUEUED | IN_PROGRESS | COMPLETED …
		Conclusion string `json:"conclusion"` // CheckRun: SUCCESS | FAILURE | NEUTRAL | SKIPPED …
		State      string `json:"state"`      // StatusContext: SUCCESS | FAILURE | ERROR | PENDING | EXPECTED
	} `json:"statusCheckRollup"`
}

func (p ghPR) toPR() *PR {
	pr := &PR{Number: p.Number, Title: p.Title, URL: p.URL, Base: p.BaseRefName, Head: p.HeadRefName,
		Conflicts: p.Mergeable == "CONFLICTING" || p.MergeStateStatus == "DIRTY",
		Blocked:   p.MergeStateStatus == "BLOCKED"}
	switch {
	case p.State == "MERGED":
		pr.State = StateMerged
	case p.State == "CLOSED":
		pr.State = StateClosed
	case p.IsDraft:
		pr.State = StateDraft
	default:
		pr.State = StateOpen
	}
	switch p.ReviewDecision {
	case "APPROVED":
		pr.Review = ReviewApproved
	case "CHANGES_REQUESTED":
		pr.Review = ReviewChangesRequested
	case "REVIEW_REQUIRED":
		pr.Review = ReviewRequired
	}
	var rs []checkResult
	for _, c := range p.StatusCheckRollup {
		name, state := c.Name, ChecksPending
		if c.Typename == "StatusContext" {
			name = c.Context
			switch c.State {
			case "SUCCESS":
				state = ChecksPassing
			case "FAILURE", "ERROR":
				state = ChecksFailing
			}
		} else if c.Status == "COMPLETED" {
			switch c.Conclusion {
			case "SUCCESS", "NEUTRAL", "SKIPPED":
				state = ChecksPassing
			default: // FAILURE, CANCELLED, TIMED_OUT, ACTION_REQUIRED, STARTUP_FAILURE, STALE
				state = ChecksFailing
			}
		}
		rs = append(rs, checkResult{name, state})
	}
	pr.Checks = summarize(rs)
	return pr
}

func (g *github) Status(ctx context.Context, branch string) (*PR, error) {
	// gh pr view <branch> only finds open PRs of the branch; list sees all
	// states (newest first) so a merged one still shows.
	out, err := g.run(ctx, "pr", "list", "-R", g.repo(), "--head", branch, "--state", "all", "--limit", "10", "--json", ghPRFields)
	if err != nil {
		return nil, err
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}
	own := prs[:0]
	for _, p := range prs {
		if !p.IsCrossRepository {
			own = append(own, p)
		}
	}
	return pickPR(own, func(p ghPR) bool { return p.State == "OPEN" }, func(p ghPR) *PR { return p.toPR() }), nil
}

// Limits of Statuses' two listings.
const (
	ghOpenLimit = 100
	ghAllLimit  = 300
	// ghLightFields: enough to name and link a pull request that is not
	// open (no checks, review or merge state, which cost the most).
	ghLightFields = "number,title,state,isDraft,url,baseRefName,headRefName,isCrossRepository"
)

// Statuses lists the repository's open pull requests (with checks) and
// its recent ones of any state, both newest first, and picks for each of
// branches what Status would: its newest open one, else its newest.
func (g *github) Statuses(ctx context.Context, branches []string) (map[string]*PR, bool, error) {
	want := make(map[string]bool, len(branches))
	for _, b := range branches {
		want[b] = true
	}
	var open, all []ghPR
	var errOpen, errAll error
	var wg sync.WaitGroup
	wg.Go(func() {
		open, errOpen = g.list(ctx, "open", ghOpenLimit, ghPRFields)
	})
	wg.Go(func() {
		all, errAll = g.list(ctx, "all", ghAllLimit, ghLightFields)
	})
	wg.Wait()
	if err := cmp.Or(errOpen, errAll); err != nil {
		return nil, false, err
	}
	byBranch := map[string][]ghPR{}
	for _, list := range [][]ghPR{open, all} {
		for _, p := range list {
			if want[p.HeadRefName] && !p.IsCrossRepository {
				byBranch[p.HeadRefName] = append(byBranch[p.HeadRefName], p)
			}
		}
	}
	out := make(map[string]*PR, len(byBranch))
	for b, prs := range byBranch {
		// The open listing comes first, with its checks.
		out[b] = pickPR(prs, func(p ghPR) bool { return p.State == "OPEN" }, func(p ghPR) *PR { return p.toPR() })
	}
	// A full open listing could hide an older open one: not complete then.
	return out, len(all) < ghAllLimit && len(open) < ghOpenLimit, nil
}

func (g *github) list(ctx context.Context, state string, limit int, fields string) ([]ghPR, error) {
	out, err := g.run(ctx, "pr", "list", "-R", g.repo(), "--state", state, "--limit", strconv.Itoa(limit), "--json", fields)
	if err != nil {
		return nil, err
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}
	return prs, nil
}

// pickPR is the first open one, else the first (lists come newest first).
func pickPR[T any](prs []T, open func(T) bool, conv func(T) *PR) *PR {
	for _, p := range prs {
		if open(p) {
			return conv(p)
		}
	}
	if len(prs) > 0 {
		return conv(prs[0])
	}
	return nil
}

func (g *github) Comments(ctx context.Context, number int) ([]Comment, error) {
	n := strconv.Itoa(number)
	out, err := g.run(ctx, "pr", "view", n, "-R", g.repo(), "--json", "url,comments,reviews")
	if err != nil {
		return nil, err
	}
	var v struct {
		URL      string `json:"url"`
		Comments []struct {
			Author    struct{ Login string } `json:"author"`
			Body      string                 `json:"body"`
			CreatedAt time.Time              `json:"createdAt"`
			URL       string                 `json:"url"`
		} `json:"comments"`
		Reviews []struct {
			Author      struct{ Login string } `json:"author"`
			Body        string                 `json:"body"`
			State       string                 `json:"state"`
			SubmittedAt time.Time              `json:"submittedAt"`
		} `json:"reviews"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}
	cs := []Comment{}
	for _, c := range v.Comments {
		cs = append(cs, Comment{Kind: "comment", Author: c.Author.Login, Body: c.Body, At: c.CreatedAt, URL: c.URL})
	}
	for _, r := range v.Reviews {
		state := strings.ToLower(r.State)
		// A "commented" review without text only holds line comments.
		if state == "commented" && strings.TrimSpace(r.Body) == "" || state == "pending" {
			continue
		}
		cs = append(cs, Comment{Kind: "review", Author: r.Author.Login, Body: r.Body, At: r.SubmittedAt, URL: v.URL, Review: state})
	}
	lines, err := g.run(ctx, "api", "--paginate", "repos/"+escapePath(g.repo())+"/pulls/"+n+"/comments?per_page=100")
	if err != nil {
		return nil, err
	}
	lc, err := decodeArrays[struct {
		User         struct{ Login string } `json:"user"`
		Body         string                 `json:"body"`
		Path         string                 `json:"path"`
		Line         int                    `json:"line"`
		OriginalLine int                    `json:"original_line"`
		HTMLURL      string                 `json:"html_url"`
		CreatedAt    time.Time              `json:"created_at"`
	}](lines)
	if err != nil {
		return nil, fmt.Errorf("gh api pulls comments: %w", err)
	}
	for _, c := range lc {
		line := c.Line
		if line == 0 {
			line = c.OriginalLine
		}
		cs = append(cs, Comment{Kind: "line", Author: c.User.Login, Body: c.Body, At: c.CreatedAt, URL: c.HTMLURL, Path: c.Path, Line: line})
	}
	sortComments(cs)
	return cs, nil
}

func (g *github) MergeMethods(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "repo", "view", g.repo(), "--json", "squashMergeAllowed,mergeCommitAllowed,rebaseMergeAllowed")
	if err != nil {
		return nil, err
	}
	var v struct {
		Squash bool `json:"squashMergeAllowed"`
		Merge  bool `json:"mergeCommitAllowed"`
		Rebase bool `json:"rebaseMergeAllowed"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("gh repo view: %w", err)
	}
	var ms []string
	for _, m := range []struct {
		ok   bool
		name string
	}{{v.Squash, "squash"}, {v.Merge, "merge"}, {v.Rebase, "rebase"}} {
		if m.ok {
			ms = append(ms, m.name)
		}
	}
	return ms, nil
}

// prHead is what Merge checks before and after: the head and the state.
func (g *github) prHead(ctx context.Context, number int) (ghPR, error) {
	out, err := g.run(ctx, "pr", "view", strconv.Itoa(number), "-R", g.repo(), "--json", "state,headRefName,isCrossRepository")
	if err != nil {
		return ghPR{}, err
	}
	var p ghPR
	if err := json.Unmarshal(out, &p); err != nil {
		return ghPR{}, fmt.Errorf("gh pr view: %w", err)
	}
	return p, nil
}

func (g *github) Merge(ctx context.Context, number int, method, head string, deleteBranch bool) (MergeResult, error) {
	var res MergeResult
	flag, err := methodFlag(method)
	if err != nil {
		return res, err
	}
	p, err := g.prHead(ctx, number)
	if err != nil {
		return res, err
	}
	if p.HeadRefName != head || p.IsCrossRepository {
		got := p.HeadRefName
		if p.IsCrossRepository {
			got += " (a fork)"
		}
		return res, headMismatch(number, head, got)
	}
	// Not --delete-branch: gh would also delete the local branch and check
	// out the base in this folder, under the user's (or the agent's) feet.
	if _, err := g.run(ctx, "pr", "merge", strconv.Itoa(number), "-R", g.repo(), flag); err != nil {
		return res, err
	}
	// A merge queue only enqueues it: deleting the branch then would close
	// the pull request.
	if p, err = g.prHead(ctx, number); err != nil {
		res.Warning = err.Error()
		return res, nil
	}
	res.Merged = p.State == "MERGED"
	if res.Merged && deleteBranch {
		// The repository may delete merged branches itself: a 422 (already
		// gone) is fine.
		if _, err := g.run(ctx, "api", "-X", "DELETE", "repos/"+escapePath(g.repo())+"/git/refs/heads/"+escapePath(head)); err != nil && !strings.Contains(err.Error(), "Reference does not exist") {
			res.Warning = err.Error()
		}
	}
	return res, nil
}

func (g *github) Create(ctx context.Context, p CreateParams) error {
	args := []string{"pr", "create", "-R", g.repo(), "--head", p.Head, "--base", p.Base, "--title", p.Title, "--body", p.Body}
	if p.Draft {
		args = append(args, "--draft")
	}
	_, err := g.run(ctx, args...)
	return err
}

func methodFlag(method string) (string, error) {
	switch method {
	case "squash", "merge", "rebase":
		return "--" + method, nil
	}
	return "", fmt.Errorf("unknown merge method %q (squash, merge or rebase)", method)
}

// escapePath escapes each segment of a/b/c for an API path.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
