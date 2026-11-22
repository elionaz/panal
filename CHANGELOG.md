# Changelog

All notable changes to panal are documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the version is 0.x, minor
releases may include breaking changes.

## [Unreleased]

### Changed

- **What 1.0 will keep stable is written down.** [docs/compatibility.md](docs/compatibility.md) now
  also covers a repository's `.panal.yaml`, the instruction topic files, `PANAL_PORT` and
  `PANAL_BROWSER_URL`, the MCP tools (names, arguments and structured fields) and the agenterm
  compatibility (kept for all of 1.x). Past-session entries now say their format version, like the
  other files: a newer panal's entry is left alone instead of being misread or pruned.

## [0.15.0] - 2026-11-20

### Added

- **panal tools for agents (MCP).** Agent sessions get panal's own MCP server, `panal`: `peers`,
  `msg_send`, `msg_read`, `wait`, `session_status`, `history_search` and `history_get`, so agents
  coordinate and look things up without running `panal` commands. It only reads and messages. Claude
  Code, codex, Gemini CLI, opencode and Cursor CLI get it for that session only (their own
  configuration is left alone); anything else can run `panal mcp`. Off with `mcp.enabled: false`
  (**Settings → General**). See [docs/mcp.md](docs/mcp.md).

### Fixed

- Adding a project shows it right away, with its buttons to start a session. Before, the session
  that was open stayed on screen.

## [0.14.0] - 2026-11-20

### Added

- **Messages between agents, open and visible.** Each project picks how its sessions' messages
  travel, with the **⇄** (automatic) / **⏸** (manual) button by its name (or
  `panal project set --messages auto|manual|default`):
  - *automatic*: any message reaches its session as soon as it can take it and wakes it when idle.
    Heads-ups count too; before, only your messages and replies did;
  - *manual*: messages are held until you deliver them, per session or all at once.

  **💬** opens the project's conversation: every message in order, each session in its own color,
  marked *waiting for you*, *on its way* or *delivered*, where you can deliver, filter and write.
  When a session writes to another, both rows in the sidebar show it for a few seconds
  (`→ fake-b: …`, `← fake-a: …`). Projects that don't choose follow `messages.auto_deliver`.

- **Past sessions, searchable.** When an agent session ends, panal keeps its name, agent, model,
  branches, the prompts you typed (Claude Code) and its terminal text as plain text, with secrets
  masked, in `~/.panal/history/`. The clock in the sidebar (⌘⇧F; Linux: Ctrl+Shift+H) searches them
  as you type, in the project or in all of them, shows one with its prompts and text, and
  **Resume** continues its conversation in a new session in the same folder. CLI:
  `panal history [words…]`, `history show|resume|rm <id>`. Settings: `history.enabled`,
  `history.keep` (300). Shells are not kept.
- **Remove merged worktrees.** Orphaned worktrees whose branch is already in its base (merged,
  squash- or rebase-merged on the host) and that have nothing uncommitted are marked
  *merged · can be removed* and listed first, and **Remove merged (N)** removes them all with
  their branches in one go. From the command line: `panal worktree rm --merged <project>`. It
  never forces: one with uncommitted changes or a newer commit is kept, with the reason.
- **Hooks for Gemini CLI, opencode and Cursor CLI.** Their sessions now report what they do
  through the agent's own hooks instead of only terminal-output guessing, set up per session without
  touching their configuration or the project (`setup_hooks: gemini`, `opencode`, `cursor`):
  - **Gemini CLI**: working, approvals ("Shell: rm -rf build"), end of turn with its reply, the
    activity line, the model, resume by session id, the standing instructions and messages from
    other sessions (like Claude Code). Through a generated system-settings file
    (`GEMINI_CLI_SYSTEM_SETTINGS_PATH`); only in folders Gemini trusts.
  - **opencode** (2 or later): a generated plugin reports each turn's start and end, failures with
    the error, approvals, questions, the activity line and the model; resume by session id; the
    standing instructions as an instructions file. Sessions run `opencode --standalone` so the
    plugin is theirs alone.
  - **Cursor CLI**: the activity line, the model, resume by chat id and the standing instructions,
    through a plugin loaded with `--plugin-dir`. Cursor doesn't run a plugin's end-of-turn hook
    yet, so working/idle stay heuristic.

### Fixed

- **`panal` is on the PATH after updating from agenterm.** The old Homebrew cask linked only the
  `agenterm` command; at launch the app now adds a `panal` link next to any `agenterm` link into
  its bundle (in `/opt/homebrew/bin`, `/usr/local/bin`, `~/.local/bin` and `~/bin`), and never
  replaces an existing `panal`.
- **Changes drop to zero as soon as the pull request is merged.** When the host reports a worktree's
  pull request as merged and the local base doesn't show it yet, panal fetches the base right away
  (it used to wait up to the next periodic fetch, every 5 minutes and only on activity), and a branch
  started from a base that was never pulled is recognized as merged too. A base whose configured
  upstream is not `origin` is now fetched as well.
- **Deleting a squash-merged branch no longer asks to force.** Removing a worktree with its branch
  counted the commits of a pull request merged with squash or rebase as unmerged; now commits whose
  changes the base already has don't count.

## [0.13.1] - 2026-11-20

### Fixed

- **The app is panal in the Dock and Finder after updating from agenterm.** In-app updates keep the
  bundle's path, so installs stayed `/Applications/agenterm.app`, and macOS shows a bundle's file
  name. At launch the app now renames itself to `panal.app` (unless one is already there) and
  reopens; the CLI symlinks pointing into the old bundle (Homebrew's `agenterm` and `panal`) are
  retargeted, and the background service is restarted from the new place like after an update
  (sessions are suspended and resumed).

## [0.13.0] - 2026-11-20

### Changed

- **agenterm is now panal.** Another project already used the name. The CLI is `panal` (`agenterm`
  still works for a while, with a notice), the app is panal.app, the per-repository file is
  `.panal.yaml` (an `.agenterm.yaml` is still read) and the variables are `PANAL_*` (`AGENTERM_*`
  still work). On its first start the daemon moves `~/.agenterm` and `~/.config/agenterm` to
  `~/.panal` and `~/.config/panal`, leaving links at the old paths, so sessions, messages, worktrees
  and settings carry over. In-app updates keep working (the bundle id stays `com.elionaz.agenterm`).
  Homebrew: `brew install --cask elionaz/tap/panal`.

### Added

- **A Linux app (experimental).** Each release has an AppImage and a .deb for amd64 and arm64
  (WebKitGTK 4.1 from the system). Shortcuts use Ctrl+Shift where macOS uses ⌘ (Ctrl+letter stays
  the shell's; split down is Ctrl+Shift+E, moving a session Ctrl+Shift+PgUp/PgDn; Ctrl+Shift+C/V copy
  and paste, Ctrl-click opens links), and labels show the Linux keys. Notifications go through the
  desktop's notification service (D-Bus), from the app (a click opens the session) and from the
  daemon; the app checks for new versions and links to the download. CI now tests and packages on
  Ubuntu too.
- **Continue with another agent.** **⇢** on a session's row hands its work to a new session (any
  agent, the same one included) in the same folder or a new worktree from its current commit, with
  a summary you can edit: the prompts you typed (Claude Code), its branch, uncommitted files and
  commits, and the end of its terminal, secrets removed. Claude Code and codex get it as their first
  prompt. Or hand it to **an existing session** (preselected when another agent works in the
  same folder), which gets it as a message from you. The old session closes unless you keep it, and
  folds under the one that continues it in the sidebar (**⇠ name**). `panal handoff` does the
  same.
- **Stopped, not failed.** A session panal ended (stop, close, continue) reads *stopped* in grey
  instead of a red `exit 129`.
- **Move to a worktree** in the *Shared folder* notice of the Changes tab: the same, preset to the
  session's agent and a new worktree, so two agents stop editing the same folder.

### Fixed

- **Deliver no longer types over your unsent text.** The Messages tab's *Deliver* / *Remind again*
  refused nothing when the session's prompt had text typed and not sent: the "new messages" line
  joined it and its Enter sent both. It now says so and waits (automatic delivery already did).
- **Cursor CLI sessions keep their chat across a daemon restart** (an update, for example): they
  come back with `cursor-agent --continue` instead of a fresh chat.
- **The daemon log says who closed a session** (`session kill requested`, with the requesting
  process and, when another session asked, its id). Before, a closed session only showed its exit
  code, which looks the same as the agent quitting on a hangup.

## [0.12.0] - 2026-11-19

### Added

- **A browser of its own for every session.** Agents no longer drive your Chrome: each session gets
  an isolated Chrome (own profile, 1280×800 window, own DevTools port) that opens only when the
  agent needs it and closes with the session. Claude Code and codex get it as browser tools
  (`chrome-devtools-mcp` through `npx`, added for that invocation only) and are told to use it;
  any agent can run `agenterm browser` or use `$AGENTERM_BROWSER_URL`. A 🌐 in the sidebar shows
  it's open. *Settings → General* turns it off or makes it headless (`browser.*`).

- **Install agents from Settings.** A catalog agent that isn't installed has an **Install** button:
  it shows the exact command, runs it in a new terminal session of the current project (output and
  any password prompt in view) and rescans the agents when it succeeds.

- **Standing instructions for the agents, by topic.** *Settings → Instructions* keeps one Markdown
  file per topic (suggested: security, architecture, data management, git), each for every project
  or only some. A topic loads **always** (its text at the start of each session) or **on demand**
  (the session only gets its name and description, and reads it with `agenterm instructions show
  <topic>` when it applies). Claude Code gets them through its SessionStart hook (again after
  `/clear`, a resume or a compaction), codex as `developer_instructions`; the agents' configuration
  and the repository are left alone. The tab says which agents receive them and previews what a
  session gets; `agenterm instructions [list]` prints the same.

## [0.11.0] - 2026-11-19

### Added

- **All the pull requests of a session in Changes.** A session that switches branches in its
  worktree and opens a pull request for each now sees them listed (*Pull requests of this session*:
  number, title, state, checks); click one to see its comments or merge it.

### Changed

- **Messages between sessions arrive by themselves.** Automatic delivery is now on by default
  (`messages.auto_deliver`; an explicit `false` in settings.yaml stays off), and besides Claude it
  reaches agents that report the end of their turn (codex), telling them to run `agenterm msg read`.
  When a message still waits, the Messages tab says why (unsent text in its input, a heads-up
  rather than a reply, an agent that doesn't report its turn end, delivery off…), or that it was
  already told, with **Deliver now** (or **Remind again**) at hand. The sidebar's count of messages
  waiting for a session is no longer amber: it is that session's, not something for you.

### Fixed

- **Changes showed the worktree's first branch**, not the one the session had switched to: the
  header, the pull request block, *Merge into the base* and *Create pull request* now follow the
  branch the worktree is on (the header says where it started), as do the sidebar and title bar.

## [0.10.0] - 2026-11-19

### Added

- **See what each session may do unasked.** A session launched to auto-approve
  (`--dangerously-skip-permissions`, codex `--full-auto` / `-a never`, `--yolo`, Cursor `--force`…),
  to auto-edit or in plan mode shows it as a chip in the title bar, with a ⚠ on the sidebar row for
  auto-approve; the tooltips now show the full command line.
- **New worktrees come ready to work.** An `.agenterm.yaml` at the root of the repository lists
  files to copy from the main folder (`worktree.copy`: `.env`, local config…) and a command to run
  before the agent starts (`worktree.setup`: `npm install`…), shown in the session's terminal. A
  setup runs only once you trust it (the app shows the command; `agenterm spawn --trust-setup` /
  `--no-setup`), and again only if it changes. Without the file, the app offers to copy the `.env*`
  files git ignores, once or into a new `.agenterm.yaml`.
- **A port per session.** Every session gets `AGENTERM_PORT`, its own port (the lowest free one
  from 3000 in the project, kept when resumed). With `ports.base` in `.agenterm.yaml`, `PORT` (or
  the variables in `ports.env`) is set too and the sidebar shows it, so two agents' dev servers
  don't fight over 3000.
- **Pick which notifications you get.** Settings → Notifications has one switch per kind:
  approvals, questions, finished turns, errors and "looks stalled" (`notify.notify_on_approval`,
  `notify_on_question`, `notify_on_done`, `notify_on_error`, `notify_on_stall`), all on by default,
  with a link to how each one is detected. Approvals, questions and errors could not be turned off
  on their own before.

### Changed

- **README: "How it works"** explains that the daemon owns the terminals (no tmux), that sessions
  survive closing the app and restarts, where attention comes from, and that worktrees are plain
  git. **"Works with any agent CLI"** now leads the feature list.

### Fixed

- **The daemon could hang** when the worktree list, removing a worktree in use, or `agenterm stop`
  checked which sessions were alive just as one changed state: the same two locks taken in opposite
  orders as the message calls fixed in 0.8.1. Found by CI; a stress test now covers it.

## [0.9.0] - 2026-11-18

### Changed

- **Automatic delivery wakes a session only for your messages and replies.** A heads-up from another
  agent (one nobody asked for) no longer wakes an idle Claude session: it waits for its next turn.
  Agents were spending turns on each other's FYIs, and answering them woke the sender back. The
  introduction agents get at start also tells them not to send heads-ups to idle sessions.
- **Sidebar rows in two lines, and a resizable sidebar.** The first line has the session's name,
  its changes and its unread messages; the state word shows only when it needs you (attention,
  waiting, exited, suspended), the dot says the rest, and ⌘N only on the selected row. The second
  line has the branch (without `agent/`, cut in the middle so its end stays readable), the model
  and the timer. Drag the sidebar's right edge to make it 220–420 px wide (double-click: back to
  248); the width is remembered.
- **Shells have no turn timer** ("finished in 3s" meant whatever ran last).
- **A tidier Changes view.** The toolbar keeps *Create pull request* and *Merge* in sight and moves
  *Open in editor*, *Open in Finder*, *Copy branch* and *Remove worktree* to a **⋯** menu; it wraps
  instead of running off the edge. Without a pull request there's no more "No pull request yet"
  strip. Files show their name first, with the folder below; commits fold. The diff has its own
  header with the path and the unified / side-by-side switch, starts unified when the panel is
  narrow (until you pick one), and wraps long lines in the unified view. The title bar drops the
  folder path (it's in the name's tooltip) and shows the branch short.

### Fixed

- **Claude Code's status line showed up as activity** (an oh-my-claudecode HUD, for example) after
  a restart or before the first hook: with hooks, output no longer sets the activity line.

## [0.8.1] - 2026-11-18

### Fixed

- **The daemon could hang** when a message call (`agenterm msg …`, the Messages tab, Claude's
  hooks) came in just as a session changed state. Two locks were taken in opposite orders, so
  every session froze until the daemon was killed. Present since session messages (0.6.0).
- **Changes counted work that was already merged.** A worktree was compared with the local base
  branch, so pull requests merged on the host showed up as the branch's changes until someone
  pulled `main`, and a squash-merged branch kept all its changes forever. Now the branch is
  compared with the base's upstream (`origin/main`) when the local copy is behind it, the daemon
  fetches that branch in the background (at most every 5 minutes, never prompting), and a branch
  whose files are already in the base shows no changes. **Merge into the base** refuses a base
  that is behind its upstream instead of making a merge commit the remote never gets.

## [0.8.0] - 2026-11-18

### Added

- **Agent sessions in new pull requests.** *Create pull request* in Changes can append an *Agent
  sessions* section to the description: the agents that worked on the branch, their model and,
  for Claude Code, the prompts typed in each session, with secrets (API keys, tokens, private
  keys, `password=…`) replaced by `[redacted]`. On by default, with a preview; untick it to leave
  it out. Prompts go in code blocks, so their links and @mentions don't render or notify.
- **Hermes Agent** in the catalog (`hermes --cli`). It shows up in the **+** menu only when the
  `hermes` binary is installed and the profile stays enabled. The approval line
  `[o]nce | [s]ession` marks the session as waiting. After a daemon restart, resume appends
  `-c` (the latest session in that directory). `args`, `resume_args` and `enabled: false`
  change or hide it; agenterm does not write `~/.hermes`.
- **The notification permission in Settings → Notifications**: whether macOS allows agenterm's
  notifications, with **Ask macOS**, **Open System Settings** (on agenterm's entry) and **Send a
  test notification**, which shows one right away or says why it can't.

### Fixed

- **Screenshots dragged from the floating thumbnail onto a terminal** reach Claude Code again. macOS
  hands over a temporary file and deletes it as soon as the drag ends, so the typed path no longer
  existed and Claude Code dropped the image without a word (Cursor Agent, which reads the path
  later, seemed to work). agenterm now keeps the file the moment it is dropped, in
  `~/.agenterm/drops/` (kept for a day), and types that path. Files dropped from Finder are typed
  as before.
- **A pending approval no longer clears itself when you look at it.** Opening a session that waits
  for an approval or an answer marked it as seen, so after switching tabs nothing said it was still
  stuck. It now stays flagged until you type into it or the agent moves on; finished turns and
  errors are still cleared once seen.
- **Cursor CLI's approval menus** (*Run this command?*, ending in *Skip & tell the agent what to do
  instead*) mark the session as waiting and notify; it showed as idle.
- **`agenterm msg send all` skips shells.** They never read messages by themselves, so a broadcast
  stayed in their Messages tab for good. Name a shell to write to it.
- **A notification permission granted after launch now applies without restarting the app.** The
  app checked it only at startup, so after allowing agenterm in System Settings its notifications
  were still dropped until a relaunch.

## [0.7.0] - 2026-11-18

### Added

- **The model of each session** next to its name in the sidebar, the title bar and split panes
  (*Opus 5.5*). Claude Code reports it through agenterm's hooks (the payload, or the end of the
  session's transcript), so a `/model` change shows up by the next turn; other agents show it when
  their profile's command line names it (`--model`, `-m`, `-c model=`). See
  [docs/attention-and-activity.md](docs/attention-and-activity.md#model).
- **The branch of every session in the sidebar**: worktree sessions keep their blue branch chip, and
  sessions in the project folder now show that folder's current branch in grey (it is shared with
  the other sessions there), also in the title bar.
- **Automatic delivery of messages** (Settings → General, `messages.auto_deliver`, off by default):
  a Claude session idle at its prompt is told about new messages without the **Deliver** click, so
  a reply that arrives after its turn ended doesn't wait for you. Never over text typed without
  sending, never while it waits for an approval, at most 5 times per session until you act on it
  again, and 20 per project every 10 minutes. See [docs/messages.md](docs/messages.md#automatic-delivery).

### Changed

- **New agents start in their own worktree by default** (`git.default_mode: worktree`), so agents
  working in parallel never share a branch or step on each other's changes; **Same folder** is one
  click away in the **+** menu. Shells always start in the project folder. A `settings.yaml` that
  already says `same` keeps it (change it in **Settings → Git**), and the per-project choice made
  before this version is no longer remembered. `agenterm spawn` is unchanged (`--worktree`).

## [0.6.0] - 2026-11-18

### Added

- **Project badges**: each project in the sidebar shows a small badge with its initials on a color
  picked from its id, also next to the title and in the overview of all sessions. Click the badge
  (or right-click the project) to rename the project, pick one of eight colors or put an emoji in
  place of the initials; `agenterm project set <project> --name/--color/--icon` does the same from
  the CLI. Notifications of running sessions use the new name.
- **Messages between the sessions of a project**: agents write to each other with
  `agenterm msg send <id|name|all>`, see who else is working with `agenterm msg peers` and wait for
  one with `agenterm wait <id>`. Claude Code gets its messages by itself through agenterm's hooks
  (an introduction at start, then new messages at each step, and a turn that would end goes on
  when one arrives); other agents read them with `agenterm msg read`. The new **Messages** tab
  lists a session's messages, lets you write to it and, for an agent idle at its prompt, **Deliver**
  types one fixed line asking it to read them (never the message). The sidebar shows an envelope while
  messages wait. The daemon decides who sent each message from the calling process, messages never
  leave their project, and **Settings → General** turns it off (`messages.enabled`). See
  [docs/messages.md](docs/messages.md).

## [0.5.0] - 2026-11-17

### Added

- **Pull requests on GitHub and GitLab in Changes**, for worktrees and for the branch the main
  folder is on: state, checks and review decision, the comments (line comments select their file),
  merging on the host (with the repository's methods and *delete the branch on the host*), and
  creating one with a title and description suggested from the commits. It runs through your own
  `gh` / `glab` login (agenterm stores no tokens); without it Changes says how to set it up and
  keeps the push-and-open-the-page flow.
- **Several agents in one project**: when an agent is already working in the project folder, the
  **+** menu, ⌘N and split panes pick a new worktree for the next one (each agent gets its own
  branch and changes) and say which session is there; **Same folder** is still one click away. Sessions that
  do share the folder say so in the Changes tab and the **±N** tooltip, and `agenterm spawn` prints
  a note on stderr. README and [docs/worktrees.md](docs/worktrees.md#several-agents-in-one-project)
  explain what the folder's sessions share.

### Changed

- **Choosing something in the sidebar closes Settings** (a session, a project, a new session, the
  overview), as × does, instead of leaving Settings covering it.
- **The overview has its own row** at the top of the sidebar (**Overview ⌘0**, highlighted while
  open) instead of a ▦ glyph in the title bar that sat off its line and said nothing. The project
  rows' overview button gets the same drawn icon.
- **After an update the app restarts the old daemon on its own** when no agent is in the middle of
  a turn: the sessions are suspended and resumed, conversations included. While an agent is
  working it still asks, with **Restart now**.

## [0.4.3] - 2026-11-17

### Added

- **Drop files on a terminal** to type their paths, escaped like Terminal.app does. Claude Code
  turns dropped images into attachments.

### Fixed

- Sessions resumed after a daemon restart (e.g. **Restart now**) no longer stay frozen in the
  app: their terminal kept the old screen and dropped every key, because the daemon told it the
  session had exited. The daemon now asks attached clients to reattach, and the app reattaches
  any terminal whose session came back with a new process.
- Shells no longer get junk like `11;rgb:0f0f/1414/2020` or `1R` typed at the prompt after the
  app reattaches: xterm answered terminal queries left in the replayed scrollback (by tools
  like `gh`) after input had already reopened. Input now waits until the replay is parsed.

### Changed

- The Homebrew cask declares `auto_updates true`: the app updates itself, so a plain
  `brew upgrade` leaves it alone. `brew upgrade --cask agenterm` (or `--greedy`) still upgrades it.

## [0.4.2] - 2026-11-17

### Changed

- **Homebrew installs update from the app**, like downloaded ones: the banner's **Update**
  installs and relaunches instead of copying `brew upgrade --cask agenterm` (which still works).
  Coming from 0.4.1 or earlier, run `brew upgrade --cask agenterm` one last time.

## [0.4.1] - 2026-11-16

### Added

- Five new terminal themes named after albums: *Is This It*, *Funeral*, *Silent Alarm* and *Z*
  (dark) and *Hot Fuss* (light).

### Changed

- Every terminal theme is now named after an album, and Settings shows only the album title.
  Solarized, Dracula, Gruvbox and Nord are gone; in `settings.yaml` their names keep working and
  pick the closest new palette: `solarized-dark` → `z`, `solarized-light` → `hot-fuss`,
  `dracula` → `funeral`, `gruvbox-dark` → `is-this-it`, `nord` → `silent-alarm`.

### Fixed

- In the light themes *It Still Moves* and *Alphabetical*, text in white or bright white was
  nearly invisible on the cream background. They're now readable grays, and bright black (dim
  text) is a darker gray so it still looks different from bright white.

## [0.4.0] - 2026-11-16

**Upgrade note:** 0.4.0 writes your settings, profiles and session files in a versioned format.
Going back to 0.3.x afterwards needs converting them back: see
[docs/compatibility.md](docs/compatibility.md#going-back-to-an-older-version).

### Added

- **Reorder by drag & drop**: drag projects in the sidebar, and sessions within their project
  (or move the selected one with ⌘⇧↑ / ⌘⇧↓). The order is kept by the daemon, so the CLI
  (`agenterm ls`, `project list`), ⌘1…⌘9 and restored sessions follow it.
- **Terminal themes named after indie albums**: *Turn On the Bright Lights* (Interpol, the new
  default), *Room on Fire* (The Strokes), *It Still Moves* (My Morning Jacket) and *Alphabetical*
  (Phoenix), with palettes taken from their covers; Settings shows the artist and year. They
  replace agenterm Dark and agenterm Light: `agenterm-dark` and `agenterm-light` in
  `settings.yaml` keep working as `bright-lights` and `alphabetical`.
- **Welcome screen** on first run (while there are no projects): the agent CLIs found (with
  Rescan), notification permission (**Allow notifications**, or **Open System Settings** if it was
  denied), whether the `agenterm` command is on your `PATH` (with the command to link the CLI that
  ships in the app), and **+ Add your first project**.
- README: how to **uninstall** (Homebrew and by hand), and what to check in `~/.agenterm/worktrees`
  before deleting your data.
- [docs/compatibility.md](docs/compatibility.md): what stays stable between versions (CLI,
  `AGENTERM_*` variables, configuration files, hooks) and how formats and the protocol are versioned.

### Changed

- **Versioned file formats**: `settings.yaml` and `profiles.yaml` get a `version` key
  (`profiles.yaml` becomes `version` + `profiles`), and `projects.json`, `worktrees.json` and
  `sessions.json` a `{"version": 1, …}` envelope. Older files are still read and get the version on
  their next save. A file written by a newer agenterm is refused with a clear error and never
  overwritten. **Going back to 0.3.x** after this needs converting the files back: see
  [docs/compatibility.md](docs/compatibility.md#going-back-to-an-older-version).
- The daemon reports its **protocol version** in `daemon.ping`. A command an older daemon doesn't
  know fails with a message that says to restart it, and the app says when the daemon is newer than
  itself instead of offering to restart (which would downgrade it).
- When the daemon can't start, the CLI and the app show why (for example, a file written by a newer
  agenterm) instead of only pointing at `daemon.log`.

### Fixed

- **Homebrew installs** can check for a new version from **Settings → General → Updates → Check
  now**, and check by themselves every 6 hours instead of once a day: a release published after
  you opened agenterm could go unnoticed for a day.

## [0.3.1] - 2026-11-16

### Fixed

- **⇧Enter** in the terminal inserts a new line in the agent's prompt (Claude Code, Codex…)
  instead of sending it. It sends a line feed (Ctrl-J); in a shell it still runs the command.

### Changed

- Development: the frontend has Vitest unit tests (`npm test`, run by CI), and the GitHub Actions
  workflows use actions that run on Node 24.

## [0.3.0] - 2026-11-16

### Added

- **Clickable URLs** in the terminal: ⌘-click opens a URL in the browser, both plain-text URLs
  and OSC 8 hyperlinks (`gh`, `ls --hyperlink`…). Only http and https; a plain click still selects.
- **Create pull request** for worktree sessions, in the Changes view and as
  `agenterm worktree pr [--no-open] <id>`: pushes the branch (`git push -u`, non-interactive, never
  forced) to the remote git would push to (`pushRemote`, `pushDefault`, upstream, `origin` or the
  only remote) and opens the GitHub compare page or the GitLab new merge request page. Other hosts
  get the push only. It doesn't need `gh`.
- After a successful **Merge into base**, the Changes view offers to remove the worktree and its
  branch.
- Credits in **About agenterm** (with the version) and a new *About* group in **Settings → General**
  with links to the author's GitHub profile and the repository.
- **Update banner**: when a new version is found, the window says so with **Update** (or
  **Restart to update** once it's downloaded; the daemon and its sessions keep running) and
  **Later**, which hides it for that version until it is downloaded. **Settings → General →
  Updates** shows it too. Scheduled checks use Sparkle's gentle reminders instead of popping its
  window while you work. Homebrew installs check the appcast daily and offer to copy
  `brew upgrade --cask agenterm`.

### Fixed

- Release notes in the update window: **bold**, links and `code` that wrap across lines of the
  changelog are now formatted too.

## [0.2.1] - 2026-11-16

### Fixed

- The release notes shown in the in-app update window now render Markdown links and *italics*
  (they appeared as raw `[text](url)` and `*text*`), and nothing inside `code` is formatted.

## [0.2.0] - 2026-11-16

### Added

- **In-app updates** with [Sparkle 2](https://sparkle-project.org): a daily background check,
  **Check for Updates…** in the app menu and an *Updates* section in **Settings → General**
  (version, automatic checks and downloads, "Check now"). Updates are EdDSA-signed and notarized.
  Homebrew installs keep updating with `brew upgrade --cask agenterm` (the app detects them and
  turns its updater off).
- After an update, a banner offers to **restart the background daemon** when it's still the
  previous version. The sessions running then are suspended and resumed whatever
  `sessions.restore` says (sessions already suspended stay so), the new daemon starts only once the
  old one has fully exited, and it asks first when agents are working. `daemon.ping` now reports
  the daemon's version.
- Release tooling: Sparkle is embedded and signed inside-out, and releases publish a signed
  `appcast.xml` (new optional `SPARKLE_PRIVATE_ED_KEY` secret and `SPARKLE_PUBLIC_ED_KEY`
  variable). Pre-release tags (`vX.Y.Z-suffix`) become GitHub pre-releases, not offered in-app nor
  to Homebrew, and, once in-app updates are on, a release without `appcast.xml` is not marked
  *Latest*.

### Fixed

- A second copy of the app with another bundle id, or started with its own `AGENTERM_HOME`, no
  longer hands off to the already open window and quits: the single-instance lock is now per
  bundle id and home.

## [0.1.1] - 2026-11-16

### Changed

- **No more Script Editor notifications by default.** When native notifications can't be used (the
  daemon isn't running from the `.app`, or notifications aren't allowed), they are no longer sent
  through AppleScript, which macOS attributes to Script Editor (and clicking one opened Script
  Editor). They are dropped and `daemon.log` says why, once. The new `notify.fallback: osascript`
  setting (**Settings → Notifications → Fallback via AppleScript**) brings the old behaviour back;
  the default is `none`.
- The CLI bundled in the app (`agenterm.app/Contents/Helpers/agenterm`, what the Homebrew cask puts
  on your `PATH`) now starts the daemon as the app's executable — on auto-start, with
  `agenterm daemon --detach` and in the foreground — so it gets native agenterm notifications and
  their clicks open the app. A CLI installed outside the bundle behaves as before.

### Fixed

- Test and demo instances can no longer pop up real notifications: `AGENTERM_NOTIFY=off` disables
  every desktop notification of the app and the daemon, whatever `settings.yaml` says, and
  `go test` never posts one (a daemon configured without a notifier now posts nothing instead of
  using osascript).

## [0.1.0] - 2026-11-16

First public release: a macOS desktop app (universal: Apple Silicon and Intel, macOS 13+) plus the
`agenterm` CLI and daemon.

### Added

- **Projects and sessions sidebar**: each project is a folder and each session a terminal
  (xterm.js) running an agent or a shell; sessions can be renamed (double-click or
  `agenterm rename`).
- **Agent catalog** with detection of the installed CLIs (Claude Code, codex, opencode, Gemini,
  Cursor, Copilot, aider, Amp, goose, Qwen, Crush, Kiro, shell), plus your own profiles and variants
  in `profiles.yaml`, editable from **Settings**.
- **Sessions outlive the app**: a daemon owns the PTYs and keeps the scrollback; the app and the CLI
  share it. Sessions are persisted: after a daemon restart they come back suspended with their
  scrollback and resume with one click (Claude Code and codex continue the same conversation).
- **Attention detection** (approvals, questions, finished turns, errors) through agent hooks and
  terminal signals, with native macOS notifications, a Dock badge and sidebar markers.
- **Live activity**: what each agent is doing, turn timers, a "stalled?" warning, progress bars
  (OSC 9;4) and a grid overview (⌘0).
- **Per-session git worktrees** with a **Changes** tab: diff, merge into the base branch and safe
  removal; also from the CLI (`agenterm diff`, `agenterm worktree …`).
- **Split panes** (⌘D / ⌘⇧D) with draggable dividers, kept per project.
- **Terminal search** (⌘F) and **appearance settings**: terminal themes, font, size, line height,
  cursor, and a light, dark or system app appearance.
- **English and Spanish** UI, following the macOS preferred languages or the one picked in
  **Settings → General**.
- `agenterm version` / `--version`.
- Release tooling: signed and notarized builds (when the credentials are configured), `.dmg` and
  `.zip` downloads, CLI archives for macOS and Linux, and a Homebrew cask
  (`brew install --cask elionaz/tap/agenterm`).

[Unreleased]: https://github.com/elionaz/panal/compare/v0.15.0...HEAD
[0.15.0]: https://github.com/elionaz/panal/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/elionaz/panal/compare/v0.13.1...v0.14.0
[0.13.1]: https://github.com/elionaz/panal/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/elionaz/panal/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/elionaz/panal/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/elionaz/panal/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/elionaz/panal/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/elionaz/panal/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/elionaz/panal/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/elionaz/panal/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/elionaz/panal/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/elionaz/panal/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/elionaz/panal/compare/v0.4.3...v0.5.0
[0.4.3]: https://github.com/elionaz/panal/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/elionaz/panal/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/elionaz/panal/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/elionaz/panal/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/elionaz/panal/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/elionaz/panal/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/elionaz/panal/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/elionaz/panal/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/elionaz/panal/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/elionaz/panal/releases/tag/v0.1.0
