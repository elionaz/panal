# Worktrees and changes

**English** · [Español](es/worktrees.md)

Each session can work in the project folder or in its own **git worktree** (a separate branch and
folder), so several agents can work on the same repo in parallel without stepping on each other.

Everything goes through the `git` CLI (with timeouts; never libgit2). Read-only queries use
`GIT_OPTIONAL_LOCKS=0`, so they never take `index.lock` while the agent is using git in the same
folder.

## Several agents in one project

Sessions in **the same folder** share everything git keeps per folder: the current branch, the
uncommitted changes and the index. What one agent edits or commits, the others see; if one switches
branch, stashes or commits, it takes the others' half-done work with it. The **±N** in the sidebar
and the Changes tab show that folder, so every session there shows the same files.

For agents working in parallel, give each one its own **worktree**. That is what panal does by
default (`git.default_mode: worktree`, since 0.7.0): a new agent starts on its own branch and
folder, and a plain **Shell** starts in the project folder. **Same folder** is still one click away
in the **+** menu.

- In a git repository, when an agent (not exited or suspended) is already working in the project
  folder, the **+** menu picks **New worktree** for the next agent and says which session is there;
  you can still pick **Same folder**. **⌘N** and splitting a pane (⌘D) do the same. The suggestion
  isn't remembered as the project's mode. A plain **Shell** neither counts nor gets the suggestion:
  a terminal next to an agent in its folder is fine.
- A session that does share the folder says so: the Changes tab shows *Shared folder* with the
  other agents, and so does the **±N** tooltip. **Move to a worktree** there continues it in a
  worktree of its own (see below).
- `panal spawn` without `--worktree` prints a note on stderr when the new session shares the
  folder (stdout is still just the id).

Each worktree's work comes back with **Merge into the base** or **Create pull request** (below).

### Continue with another agent

A running process can't change folders, so a session is never "moved": its work is **continued** by
a new session. **⇢** on a session's row (or **Move to a worktree** in a shared folder's Changes tab)
opens *Continue with another agent*:

- **Continue in:** a new session, or **an existing session** of the project that takes the work
  over (preselected when another agent works in the same folder): it gets the summary as a message
  from you (see [messages.md](messages.md)), and nothing new starts.
- **Agent** (new session): any installed one, the same included.
- **Where** (new session): the same folder or worktree, or **a new worktree** on a new branch from the session's
  current commit (its commits come along; uncommitted changes stay where they are). In a shared
  folder the new worktree is preselected.
- **Summary:** drafted from the session (the prompts you typed, for Claude Code; its folder and
  branch, uncommitted files and commits; the end of its terminal), with secrets removed. Edit it
  before continuing.

The new session gets the summary as its first prompt (Claude Code, codex) or, for other agents, a
line typed at its prompt pointing to it as a file (`~/.panal/handoffs/<sessionId>.md`). It keeps
the old session's name. The old session closes, so two agents don't edit the same files, unless you
tick *Keep this session open*. Once closed, it folds under the session that continues it in the
sidebar (**⇠ name** on that row shows it), and a session stopped by panal reads *stopped*, not a
red exit code. From the CLI:
`panal handoff [--worktree] [--keep] [--note FILE|-] <sessionId> <profile>`,
`panal handoff --into <sessionId> <sessionId>` (an existing session), `--print` shows the summary.

## Session modes

- **Same folder** (`same`, the default): the agent works in the project folder.
- **New worktree** (`worktree`): `git worktree add -b <branch> <folder> <base>`.
  - **Branch**: `agent/<profile>-<first 4 characters of the session id>` by default. The name is
    sanitized (spaces and invalid characters → `-`, no `..`, no `.lock`…) and, if it already exists,
    gets `-2`, `-3`… appended.
  - **Base**: the project's current branch by default (or `main`/`master` if `HEAD` is detached).
    It can be any branch or commit.
  - **Folder**: `<worktrees>/<project>-<projectId>/<sessionId>/`, where `<worktrees>` is
    `~/.panal/worktrees` (or `git.worktree_dir`) and `<project>` is the project name, lowercased
    and sanitized. Worktrees are **never** created inside the project.
  - It's allowed even if the main folder has changes: the worktree starts from the base's commit.
  - If the project is a subfolder of the repo, the session starts in that same subfolder of the
    worktree.
  - Clear errors if the project isn't a git repo (in the app the option is disabled, with the
    reason), if the repo has no commits yet or if the base doesn't exist.
- **Reopening a worktree** (`--in-worktree <id>` in the CLI, *open a shell here* in the app): a new
  session in an existing panal worktree.

A worktree's id is the id of the session that created it.

Hook integrations work the same way in a worktree: Claude Code gets `--settings` with an absolute
path and nothing is written to the worktree or the project. Claude Code may ask you to trust the new
folder the first time.

## Setting up new worktrees

A new worktree only has what git tracks, so an agent's first run often fails: no `.env`, no
`node_modules`. A **`.panal.yaml`** at the root of the repository (commit it to share it) says
what every new worktree gets:

```yaml
worktree:
  copy: [.env, .env.local, config/local.json]   # from the main folder; simple globs allowed
  setup: npm install                             # runs in the new worktree before the agent
```

- **`copy`**: files or folders, relative to the repository root, copied from the main folder right
  after `git worktree add`, keeping their paths. Nothing is overwritten (a file git already put
  there stays), missing entries are skipped, and nothing may point outside the repository
  (absolute paths, `..`, `.git` and symlinks that leave it are refused). On macOS files are
  cloned (APFS), so even a large folder costs almost nothing.
- **`setup`**: a shell command run in the session's terminal, in your login shell, before the
  agent starts; you see its output there. If it fails, the terminal says so and waits: **Enter**
  starts the agent anyway, **Ctrl-C** ends the session.
- **Trust**: a repository can carry any command, so panal runs a `setup` only once you trusted
  it. The first time (and whenever the command changes) the app shows it and asks: **Trust and
  create**, **Create without setup** or cancel. From the CLI: `panal spawn --worktree
  --trust-setup` or `--no-setup` (without either, the spawn fails with `setup_untrusted` and the
  command). What you trusted is kept per repository in `~/.panal/trust.json` (only a hash of
  the command).
- **Only new worktrees**: resuming a session or reopening a worktree runs nothing again.
- **`node_modules`**: let `setup` install it (`npm install`, `pnpm i`…) rather than copying it:
  branches can have different dependencies, and a shared folder would mix them.
- **Without the file**: when a repository has no `.panal.yaml` and git ignores `.env*` files at
  its root, the app offers to copy them into the new worktree, once or by writing a first
  `.panal.yaml` with them (`panal spawn` prints a note instead).

## A port per session

Two agents that start a dev server both want port 3000. Every session gets **`PANAL_PORT`**, a
port of its own: the lowest one from 3000 that no other session of the project holds and nothing
listens on. A session keeps it when it is resumed. When the repository's `.panal.yaml` asks for
ports, **`PORT`** is set to it too, and the sidebar and title bar show it (`:3001`; click it to copy
`localhost:3001`):

```yaml
ports:
  base: 3000            # first port tried
  env: [PORT]           # variables set to the port (default PORT)
```

Next.js, Express, Rails, Flask and most servers read `PORT`. **Vite doesn't**: start it with
`vite --port $PANAL_PORT` (in your `dev` script), or read `process.env.VITE_PORT` in
`vite.config` and list it: `env: [PORT, VITE_PORT]`. Without `ports.base`, only `PANAL_PORT` is
set, so nothing changes for servers that expect their usual port.

## Lifecycle

- **Killing a session never deletes its worktree.** The registry (`worktrees.json`) survives daemon
  restarts.
- Worktree states (`panal worktrees <project>`):
  - `live`: a live session is using it;
  - `exited`: only used by sessions that have exited but are still in the list, or that are
    `suspended` after a daemon restart (resuming one reopens it in this worktree);
  - `orphan`: no session in the daemon uses it (you removed or discarded them, or they had
    already exited before the daemon restarted).
- When listing a project's worktrees, the daemon forgets the ones that no longer exist on disk (and
  runs `git worktree prune`) and **adopts** worktrees git knows about inside that project's
  worktrees folder that the registry doesn't have (for example, if `worktrees.json` was lost).

## Changes (diff)

- **Worktree**: `git diff <merge-base with the base>` (committed and uncommitted changes, in a
  single list) plus untracked, non-ignored files, and the commits ahead (`git log <base>..HEAD`, at
  most 200). If the base branch disappears, the base commit saved when the worktree was created is
  used.
- **The base is kept current**: when the local base branch is behind its upstream (`origin/main`:
  pull requests merged on the host and never pulled) and the branch already has those commits, it
  is compared with the upstream instead, so they don't count as the branch's changes. The daemon
  fetches that upstream branch in the background (`git fetch <remote> <base>`, at most every 5
  minutes per repository, never prompting for credentials: offline or without a credential it just
  doesn't). A branch whose pull request was **squash- or rebase-merged** (its files are already the
  same in the base) shows no commits or changes, only what's left uncommitted.
- **Same folder**: `git diff HEAD` plus untracked files (against the empty tree if the repo has no
  commits).
- Per file: path, previous path (renames), status `A`/`M`/`D`/`R`/`C`/`T`/`U`, `+`/`−` lines, whether
  it's binary (no content) and its patch.
- Limits: **2 MB** of patch in total (remaining files are listed without content and the diff is
  marked `truncated`); untracked files over 1 MB, or beyond the first 300, don't include content
  either.
- Each session's **summary** (`git`: `files`, `uncommitted`, `ahead`) is recomputed lazily (on going
  idle, waiting or exited, and when the diff is requested), at most once every 3 s per session and
  never on every chunk of output. Changes are published with the `session.git` event.

## Actions

- **Merge into the base** (`panal worktree merge <id>`): `git merge --no-ff` of the branch in the
  repo's **main folder**. Only if:
  - the main folder is on the base branch;
  - it has no changes to tracked files (untracked files don't block it; there's no automatic
    stash);
  - the base isn't behind its upstream (pull it first: a merge into a stale base makes a merge
    commit the remote never gets);
  - the branch has commits the base doesn't have.

  On conflicts it runs `git merge --abort` and reports which files clashed. Uncommitted changes in
  the worktree aren't part of the merge (you're told how many were left out). In the app, a
  successful merge then offers to remove the worktree and its branch (the usual remove
  confirmation, with *delete the branch* already ticked).
- **Create pull request** (`panal worktree pr [--no-open] <id>`): `git push -u <remote> <branch>`
  and then opens the host's page to create the pull request into the base. It doesn't use `gh` and
  never force-pushes or pushes the base.
  - Remote: the one git would push to: `branch.<branch>.pushRemote`, else `remote.pushDefault`,
    else the branch's upstream remote, else `origin`, else the only remote. It refuses if there is
    none (or several and none of these applies).
  - Refuses if the branch has no commits the base doesn't have.
  - The push is non-interactive (`GIT_TERMINAL_PROMPT=0`, ssh in `BatchMode`): if credentials
    would be asked for, it fails with git's message instead of waiting. Credential helpers and
    `ssh-agent` keep working. Unreachable or stalled servers are given up on (ssh connect and
    keep-alive timeouts, a minimum http speed) and the whole push is limited to two minutes.
  - The page: GitHub (`https://github.com/<owner>/<repo>/compare/<base>...<branch>?expand=1`) and
    GitLab (gitlab.com and hosts whose name contains `gitlab`: *new merge request*), from http(s),
    ssh or `git@host:path` remote URLs (`ssh.github.com` and ssh aliases like `github.com-work`
    count as GitHub). Other hosts: the branch is pushed and you open the pull
    request there yourself.
  - Uncommitted changes in the worktree are not pushed (you're told how many).
- **Remove worktree** (`panal worktree rm <id>`):
  - refuses if a live session is using it;
  - without *force* it also refuses if uncommitted changes would be lost or, when also deleting the
    branch (`--delete-branch`), commits not merged into the base; the error lists what would be
    lost;
  - `--dry-run` only runs the check;
  - after removing: `git worktree prune` and, if requested, `git branch -D` (safe: unmerged commits
    were already checked against the base, unless *force* is used);
  - commits whose changes the base already has (a pull request squash- or rebase-merged on the
    host) don't count as unmerged: deleting a merged branch needs no *force*.
- **Remove merged** (`panal worktree rm --merged [--dry-run] <project>`): removes, with their
  branches, every orphaned worktree whose branch is already merged into its base (by a merge, a
  squash or a rebase) and that has nothing uncommitted. It never forces: a worktree with
  uncommitted changes, or with a commit made since, is kept and listed with the reason.
- **Open in editor** (`git.editor`; `code` by default if it's on the `PATH`, otherwise Finder) and
  **Open in Finder** (`panal worktree open [--finder] <id>`).
- **Copy branch** (app only, to the clipboard).

## Notes for the agent

In a live agent session's **Changes**, click a line number of the diff to leave a note on that line
for the session's agent (shift-click another line of the same file for a range; both views work,
and removed lines are named as lines of the old version). The notes gather above the file list,
marked in the diff; **Send to &lt;session&gt;** sends them all as one message from you, with each line's
place (`file:line`) and code quoted, and the agent reads it like any other
[message](messages.md). Notes not sent yet stay while the app runs, per session; quitting the app
drops them. Shells and exited sessions don't take notes.

## Pull requests on GitHub and GitLab

The **Changes** tab shows the pull request of the branch, for worktree sessions and for sessions in
the main folder (the branch it is on; nothing on the base branch). It goes through the host's own
CLI with your login: [`gh`](https://cli.github.com) for GitHub and
[`glab`](https://gitlab.com/gitlab-org/cli) for GitLab (gitlab.com and hosts whose name contains
`gitlab`). panal stores no tokens. Without the CLI, or logged out, Changes says how to set it up
(`brew install gh`, `gh auth login`) and keeps the push-and-open-the-page flow above.

**A session that works on several branches.** An agent may switch branches inside its worktree
(`git switch -c fix/…`) and open a pull request for each. Changes follows the branch the worktree is
on now (the header says which one it started on), and *Merge into the base* and *Create pull
request* act on it. The worktree remembers every branch it was on (or pushed from), and when there
is more than one pull request, Changes lists them all, *Pull requests of this session*, with their
state and checks: click one to see it below (comments, merge on the host). A new pull request is
only offered for the branch the worktree is on.

- **State**: number, open / draft / merged / closed, checks (passing, failing with their names,
  running; GitLab: the pipeline) and the review decision. The title links to the host.
- **Comments**: conversation comments, reviews with text (approved, changes requested) and line
  comments with `file:line`, which select that file in the diff.
- **Send to the agent** (live agent sessions): *Ask &lt;session&gt; to fix them* next to failing checks
  sends a message naming them and how to look at them (`gh pr checks`, `glab ci status`); in the
  comments, *Send to &lt;session&gt;* on one comment or *Send all to &lt;session&gt;* sends them with their
  author, `file:line` and link, asking the agent to address them and push.
- **Merge on the host**: with the methods the repository allows (squash, merge commit, rebase) and
  *delete the branch on the host afterwards*. Warnings for drafts, conflicts, failing or running
  checks and merges the host may refuse; the host has the last word. Local folders and branches are
  never touched (`gh pr merge` runs without `--delete-branch`, which would check out the base in
  your folder). The merge is refused if the folder moved to another branch since the pull request
  was shown, or if it is a fork's branch of the same name (forks' pull requests are never shown as
  yours). When the host only queues it (a merge queue, GitLab's auto-merge once the pipeline
  passes), Changes says so and the branch is kept until it really merges.
- **Create pull request**: pushes the branch (never forced) and opens it into the base (the
  worktree's base, or the remote's default branch for the main folder) with an editable title and
  description suggested from the commits, optionally as a draft.
- **Add the agent sessions** (on by default, with a preview): appends an *Agent sessions* section
  to the description with the agents that worked in that folder (shells left out), their model
  and, for Claude Code, the prompts you typed in each session (read from its transcript; tool
  output, subagents and slash commands left out). Prompts go in code blocks, so links and
  @mentions in them don't render or notify anyone. What looks like a secret (API keys, tokens,
  private keys, `password=…`) is replaced with `[redacted]` — a best-effort filter: check the
  preview before creating. Untick it to leave the section out.

Everything reloads when Changes opens, with ⟳ and when the branch moves (commits, pushes); there is
no polling. The CLIs are found through the login shell's `PATH`, like the agents. The repository
is the one the branch is pushed to: in a fork setup where that is your fork, pull requests are
looked up (and opened) there. Comments are read in pages of 100, all of them.

## In the app

- A project's **+** menu: after picking the agent, choose same folder or new worktree (an editable,
  pre-filled branch and a base selector defaulting to the current branch). The last choice is
  remembered per project (for agents; shells always start in the folder), and ⌘N uses it; if
  there's none, `git.default_mode` is used (`worktree` by default). Choices made before 0.7.0 are
  not remembered any more.
- Session row: a chip with the branch (without the `agent/` prefix): blue for the session's own
  worktree, grey for the project folder's current branch (shared with the other sessions there);
  then `±N` changed files. The same on the overview cards, with `↑N` commits ahead.
- **Terminal** and **Changes (N)** tabs on sessions with git: a file list on the left (status,
  `+`/`−`), commits ahead, the diff on the right with diff2html (side by side or unified), ⟳ to
  reload, a truncated-diff notice and the action bar (open in editor, open in Finder, copy branch,
  merge into &lt;base&gt;, create pull request (a confirmation with the remote, branch and base),
  and remove worktree with a confirmation that shows what would be lost, a
  checkbox to also delete the branch and another to force).
- Under each project, **orphaned worktrees** (if any): view changes, open a shell here (a new
  `shell` session in that worktree) and remove. Those already merged and clean come first, marked
  **merged · can be removed**, and the title offers **Remove merged (N)**, which asks once and
  removes them with their branches.
