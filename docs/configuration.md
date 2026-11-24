# Configuration

**English** · [Español](es/configuration.md)

## Paths

| | default | with `PANAL_HOME=/x` |
|---|---|---|
| daemon socket, pid and log | `~/.panal/{d.sock,d.pid,daemon.log}` | `/x/…` |
| profiles, projects and settings | `~/.config/panal/{profiles.yaml,projects.json,settings.yaml}` | `/x/…` |
| standing instructions for the agents (one file per topic) | `~/.config/panal/instructions/<topic>.md` | `/x/instructions/…` |
| each session's own browser profile | `~/.panal/browsers/<sessionId>/` | `/x/browsers/…` |
| generated hook files | `~/.panal/hooks/` | `/x/hooks/` |
| session worktrees | `~/.panal/worktrees/<project>-<projectId>/<sessionId>/` (or `git.worktree_dir`) | `/x/worktrees/…` |
| worktree registry | `~/.panal/worktrees.json` | `/x/worktrees.json` |
| sessions to restore and their scrollback | `~/.panal/sessions.json`, `~/.panal/scrollback/<id>.log` | `/x/…` |
| messages between sessions | `~/.panal/messages.json` | `/x/messages.json` |
| past sessions | `~/.panal/history/<id>.{json,txt}` | `/x/history/…` |

- Directories are created with `0700` permissions and the socket with `0600`.
- On macOS a unix socket path is limited to about 104 bytes: if you use `PANAL_HOME`, keep it
  short (`/tmp/agt`, not a long temporary folder under `/var/folders/…`).
- `profiles.yaml` and `settings.yaml` are optional. When the app (or an RPC) saves them, the write
  is atomic and leaves the previous version in `<file>.bak`.

## Agents: catalog and detection

panal ships a catalog of known agent CLIs. An agent counts as **installed** if its binary is
found in the daemon's `PATH` or in your login shell's (`$SHELL -l -i -c`); that lookup is cached for
5 minutes. `panal profiles scan` or the rescan button in **Settings → Agents** refreshes it.

**Installing from the app**: a catalog agent that isn't installed has an **Install** button in
**Settings → Agents**. It shows the exact command first (several are `curl … | bash`), then runs it in
a new terminal session of the current project, in your login shell, so you see its output and can
type a password if it asks; the shell stays open afterwards. A successful install rescans the agents.
Only the catalog's own commands are run (the first one of each hint; the alternatives are shown as
text), never a value from `profiles.yaml`.

| id | agent | command | status | hooks | notes |
|---|---|---|---|---|---|
| `claude` | Claude Code | `claude` | `hooks` | `claude` | |
| `codex` | Codex CLI | `codex` | `heuristic` | `codex` | `notify` only reports the end of a turn, so "working" is decided by the heuristic |
| `opencode` | opencode | `opencode` | `hooks` | `opencode` | a plugin reports every turn (start, end, failure), approvals and questions; needs opencode 2 |
| `gemini` | Gemini CLI | `gemini` | `heuristic` | `gemini` | no hook for a failed or cancelled turn, so the heuristic brings those back to idle |
| `cursor-agent` | Cursor CLI | `cursor-agent` | `heuristic` | `cursor` | the CLI runs no turn-end hook from plugins; attention patterns `Run this command\?` and `Skip & tell the agent what to do instead` (its approval menus) |
| `copilot` | GitHub Copilot CLI | `copilot` | `heuristic` | | |
| `aider` | Aider | `aider` | `heuristic` | | attention pattern `\(Y\)es/\(N\)o` |
| `amp` | Amp | `amp` | `heuristic` | | |
| `goose` | goose | `goose session` | `heuristic` | | |
| `qwen` | Qwen Code | `qwen` | `heuristic` | | |
| `crush` | Crush | `crush` | `heuristic` | | |
| `kiro` | Kiro CLI | `kiro-cli chat` | `heuristic` | | binary name not verified |
| `hermes` | Hermes Agent | `hermes --cli` | `heuristic` | | attention pattern `\[o\]nce\s+\|\s+\[s\]ession`; resume appends `-c` (latest session in that directory). Only launches when `hermes` is installed and the profile is enabled; `args` / `resume_args` are overridable |
| `shell` | Shell | `$SHELL` (or `/bin/sh`) | `heuristic` | | always available |

- **status** (`status`): `hooks` = state is decided by explicit events (hooks / `notify`) and the
  heuristic is only used until the first event; `heuristic` = output → working, a few seconds
  without output → idle. See [attention-and-activity.md](attention-and-activity.md).
- **hooks** (`setup_hooks`): the integration panal sets up when launching the agent.
- Agents that aren't installed (or are disabled) don't appear in the **+** menu or on ⌘N; they do
  appear in **Settings**, with the install command and a link.

## `profiles.yaml`

A `version` key (the [file format](compatibility.md#file-formats)) and a `profiles:` list (files
from before 0.4.0 are just the list, and are still read). Each entry has a required, unique `id` and is **layered over** the catalog:

| field | |
|---|---|
| `id` | identifier (letters, digits, `.`, `_`, `-`) |
| `name` | display name |
| `extends` | id of the profile it inherits from (custom profiles only) |
| `enabled` | `false` hides it from the menus; `true` by default (not inherited) |
| `order` | explicit position (lowest first); without it, the position in the list counts |
| `cmd` | command; `$VARIABLES` are expanded |
| `args` | list of arguments |
| `env` | `KEY: value` map (values also expand `$VARIABLES`) |
| `status` | `hooks` or `heuristic` |
| `setup_hooks` | `claude`, `codex`, `gemini`, `opencode`, `cursor` or `none` |
| `idle_seconds` | seconds without output before going idle (overrides the daemon's value, 3 s) |
| `attention_patterns` | regular expressions (Go syntax) over recent output with ANSI stripped; a match marks the session as waiting for your reply |
| `resume_args` | arguments appended (after `args`) when a session of this profile is resumed after a daemon restart and there is no agent session id to resume; e.g. `["--continue"]`. The catalog sets it for opencode and Cursor CLI (`--continue`) and Hermes (`-c`): the project's last session. claude, codex, gemini, opencode and Cursor CLI resume by id (with their integration) and only use it as a fallback. Not shown in the app's editor (saving from the app keeps it) |

### Merge rules

- **Entry with a catalog agent's id**: changes only the fields you set; the rest (including
  `setup_hooks`) is inherited. An empty or omitted field inherits; `setup_hooks: none` disables the
  integration; `args: []` clears the inherited arguments. A catalog entry can't use `extends`.
- **New id without `extends`**: a custom profile; `cmd` is required.
- **New id with `extends: <id>`**: a variant. It inherits everything from the already-resolved
  parent (your changes included) and its own fields are applied on top; `args`, `env` and
  `attention_patterns` **replace** the parent's (they aren't merged). A variant's default name is
  its id.
- **Order**: file entries first, by `order` or by position; then the catalog agents the file
  doesn't mention, in catalog order. The first enabled, installed one is what ⌘N launches. An entry
  with just `- id: x` pins a catalog agent's position.
- **Errors**: an error in one entry (invalid regex, unknown or cyclic `extends`, missing `cmd`,
  unknown `setup_hooks`…) only disables that entry; it shows up in `panal profiles` and in
  **Settings**. Structural errors (invalid YAML, empty or duplicate id) invalidate the whole file:
  commands that read it fail until you fix it.

```yaml
version: 1
profiles:
  - id: claude-yolo           # variant
    name: Claude (no permissions)
    extends: claude
    args: ["--dangerously-skip-permissions"]
  - id: claude
    env: {CLAUDE_CODE_USE_BEDROCK: "1"}
  - id: gemini
    enabled: false
  - id: mine                  # custom profile
    cmd: $HOME/bin/my-agent
    args: []
    env: {}
    status: heuristic         # hooks | heuristic
    setup_hooks: none         # claude | codex | gemini | opencode | cursor | none
    idle_seconds: 3
    attention_patterns:
      - '\(y/n\)'
```

### Editing from the app

**Settings → Agents** (⌘,, the ⚙ in the sidebar or the **panal** app menu) lets you enable and
disable agents, reorder them (drag or ▲▼), edit them (name, command, arguments, `KEY=value`
environment, status mode, hook integration, idle seconds and attention patterns), duplicate one as a
variant, add a custom agent, delete custom profiles, reset modified catalog agents and rescan.

When the app saves (RPC `profile.save`, `profile.delete`, `profile.reorder`), the file is rewritten
in **canonical form**: one entry per profile in menu order, only with the fields that differ from
what's inherited (unchanged catalog agents become `- id: x`) and without `order`. **Comments are not
preserved.** The file is re-read on every `spawn` and every listing, so you can also edit it by
hand.

## `settings.yaml`

Every key is optional; missing ones take the default value.

| key | default | |
|---|---|---|
| `ui.language` | `auto` | app language: `auto`, `en` or `es` (see [Language](#language)) |
| `ui.appearance` | `system` | the app's colors (sidebar, title bars, Settings): `system` follows macOS light/dark, or `dark` / `light`. Terminals keep their own `terminal.theme` |
| `terminal.font_family` | `"SF Mono", Menlo, Monaco, monospace` | a CSS font-family list; the first installed font is used. Empty: the default |
| `terminal.font_size` | `13` | points, clamped to 8–32. ⌘= / ⌘- / ⌘⌥0 change it (and save it) from anywhere in the app |
| `terminal.line_height` | `1.15` | a multiple of the font size, 1.0–2.0 |
| `terminal.cursor_style` | `block` | `block`, `bar` or `underline` |
| `terminal.theme` | `bright-lights` | built-in palette, each named after an album. Dark: `bright-lights` (*Turn On the Bright Lights*), `room-on-fire` (*Room on Fire*), `is-this-it` (*Is This It*), `funeral` (*Funeral*), `silent-alarm` (*Silent Alarm*), `z` (*Z*). Light: `it-still-moves` (*It Still Moves*), `alphabetical` (*Alphabetical*), `hot-fuss` (*Hot Fuss*). Retired names still work and pick the closest palette: `panal-dark` → `bright-lights`, `panal-light` → `alphabetical`, `solarized-dark` → `z`, `solarized-light` → `hot-fuss`, `dracula` → `funeral`, `gruvbox-dark` → `is-this-it`, `nord` → `silent-alarm`. Unknown names fall back to `bright-lights` |
| `notify.enabled` | `true` | notifications (from the app and the daemon) |
| `notify.sound` | `true` | sound, only for approvals and questions |
| `notify.notify_on_approval` | `true` | notify when an agent asks for an approval |
| `notify.notify_on_question` | `true` | notify when an agent asks you something |
| `notify.notify_on_done` | `true` | notify when an agent finishes its turn |
| `notify.notify_on_error` | `true` | notify when a turn fails |
| `notify.notify_on_stall` | `true` | low-key notice when a turn looks stalled |
| `notify.fallback` | `none` | when native notifications can't be used (no `.app` bundle, no permission): `none` doesn't show them (`daemon.log` says why, once) and `osascript` sends them through AppleScript, which macOS shows as *Script Editor* (clicking opens Script Editor, not panal). `PANAL_NOTIFY=off` in the environment disables all notifications regardless. See [attention-and-activity.md](attention-and-activity.md#while-the-app-is-closed) |
| `activity.stall_seconds` | `180` | seconds without output or events before marking "stalled?"; `0` disables it |
| `git.editor` | `""` | for "Open in editor": a command on the `PATH` (`code`, `cursor -n`, `zed`), an absolute path or the name of a macOS app (`Visual Studio Code`, `Cursor.app`). Empty: `code` if it's on the `PATH`, otherwise Finder |
| `git.worktree_dir` | `""` | where to create worktrees (absolute path or `~/…`). Empty: `~/.panal/worktrees` (or `$PANAL_HOME/worktrees`) |
| `git.default_mode` | `worktree` | `worktree` or `same`: where the **+** menu (and ⌘N) starts a new agent in a project where you haven't chosen yet. Shells always start in the project folder. Before 0.7.0 the default was `same`; a file that says `same` keeps it |
| `sessions.restore` | `ask` | what a starting daemon does with the sessions that were live when the previous one stopped: `ask` (keep them `suspended` until you resume or discard them), `always` (resume them right away) or `never` (discard them). See [architecture.md](architecture.md#persistence-and-resume) |
| `messages.enabled` | `true` | messages between the sessions of a project (`panal msg`): when off, sending is refused and agent hooks hand nothing over. See [messages.md](messages.md) |
| `messages.auto_deliver` | `true` | the messages mode of projects that haven't chosen one (each project switches with the ⇄ / ⏸ button in the sidebar or `panal project set --messages`): `true` is automatic (a message reaches its session as soon as it can take it, waking it when idle, never over text typed without sending, at most 5 times per session until you act on it again and 20 per project every 10 minutes), `false` is manual (messages wait for you to deliver them). See [messages.md](messages.md#automatic-or-manual-per-project) |
| `browser.enabled` | `true` | each session gets its own isolated Chrome; see [Browser for agents](#browser-for-agents) |
| `mcp.enabled` | `true` | agent sessions get panal's MCP server (peers, messages, wait, status, history); see [mcp.md](mcp.md) |
| `browser.headless` | `true` | run those browsers without a window (you watch them in the session's **Browser** tab) |
| `browser.path` | `""` | a Chrome-compatible executable; empty: Google Chrome, Chromium, Microsoft Edge or Brave, whichever is installed |
| `history.enabled` | `true` | keep ended agent sessions, searchable; see [Past sessions](#past-sessions) |
| `history.keep` | `300` | how many ended sessions are kept (1–5000); the oldest go first |

```yaml
version: 1
ui:
  language: auto
  appearance: system
notify:
  enabled: true
  sound: true
  notify_on_approval: true
  notify_on_question: true
  notify_on_done: true
  notify_on_error: true
  notify_on_stall: true
  fallback: none
activity:
  stall_seconds: 180
git:
  editor: ""
  worktree_dir: ""
  default_mode: worktree
sessions:
  restore: ask
messages:
  enabled: true
  auto_deliver: true
terminal:
  font_family: '"SF Mono", Menlo, Monaco, monospace'
  font_size: 13
  line_height: 1.15
  cursor_style: block
  theme: bright-lights
```

When it's read:

- `notify.*` is re-read on every notification (in the app and in the daemon).
- `activity.stall_seconds` is read when each session is launched: it affects new sessions.
- `git.worktree_dir` is read when each worktree is created (and when listing a project's worktrees).
- `git.editor` is read on every "Open in editor".
- `sessions.restore` is read when the daemon starts.
- `messages.enabled` is read on every message sent and every hook that could deliver one;
  `messages.auto_deliver` whenever a message arrives or a session goes idle.
- `ui.appearance` and `terminal.*` are read by the app when it loads (and when Settings opens);
  changes made in the app apply at once to every open terminal. A hand edit applies on the next
  reload of the window.

**Settings → Notifications** edits the `notify.*` keys and `activity.stall_seconds` (RPC
`settings.save`). Saving edits the file in place: unknown keys and comments **are preserved**, and
fields a client doesn't send keep their value. If the file can't be parsed, the defaults are used
(and this is logged); saving replaces it.

**Settings → Git** edits `git.editor`, `git.worktree_dir` and `git.default_mode`. **Settings →
General** edits `ui.language`, `sessions.restore`, `messages.enabled`, `messages.auto_deliver`, `browser.enabled`, `browser.headless`, `mcp.enabled` and `history.enabled` (saved right away). **Settings →
Appearance** edits `ui.appearance` and `terminal.*`, with a live preview; changes apply to the open
terminals at once and are saved right away.

### Language

`ui.language` sets the app language:

- `auto` (default): the first of English/Spanish found in the macOS preferred languages, then in
  `LC_ALL` / `LC_MESSAGES` / `LANG` (`C`/`POSIX` are skipped); otherwise English;
- `en`: English;
- `es`: Spanish.

You can pick it in **Settings → General**; it localizes the app UI, its menu and notifications. CLI
output and daemon/RPC error messages are always in English; user-facing refusals carry stable error
codes so the app can show them in its language. Any other value is treated as `auto`.

## Standing instructions for the agents

**Settings → Instructions** holds instructions that panal hands the agents it launches when each
session starts, organized in topics: security, architecture, data management, git, or any other.
Use them for your own rules ("never force-push", "use pnpm", "for browser tests use the panal
browser, not my Chrome"); instructions your team shares belong in the repository's `AGENTS.md` /
`CLAUDE.md`.

Each topic is a Markdown file in `~/.config/panal/instructions/<topic>.md`, private (`0600`) and
outside the repository, with a front matter you can also edit by hand:

```markdown
---
title: Git
description: Branches, commits, pull requests and merges
load: on_demand      # or always
projects: all        # or [<projectId>, …]
enabled: true
order: 40
---

- Work on a feature branch, never on main.
- Never force-push.
```

A file without front matter loads always, in every project.

- **Load.** An `always` topic goes in full into the session at start. An `on_demand` topic only
  shows its name and description there; the agent runs `panal instructions show <topic>` to read
  it when the work touches it (like a skill), so long topics don't take context in every session.
- **Projects.** A topic applies to every project or only to the ones it lists (in the app: *only
  some*); projects removed since are ignored.
- **Limits.** 16 KiB per topic; at most 32 KiB of `always` text per session (past that, the
  remaining topics are listed to read on demand). The tab shows the size of what a session gets
  and warns past 8 KiB.
- **Preview.** The tab shows the exact text a session of a project receives;
  `panal instructions [<project>]` prints it too, `panal instructions list` lists the topics.
- **Claude Code** (`setup_hooks: claude`) gets them in its `SessionStart` hook, so again after
  `/clear`, a resume or a compaction; also with `messages.enabled: false`.
- **codex** (`setup_hooks: codex`) gets them as `-c developer_instructions=…` for that invocation.
  If your `~/.codex/config.toml` (or `$CODEX_HOME/config.toml`) sets `developer_instructions`
  itself, panal leaves it alone and codex sessions don't get these (the tab says so).
- **Gemini CLI** (`setup_hooks: gemini`) gets them in its `SessionStart` hook (start, resume,
  `/clear`), in folders Gemini trusts.
- **opencode** (`setup_hooks: opencode`) gets them as an instructions file
  (`hooks/instructions/<project>.md`) listed in its per-session configuration.
- **Cursor CLI** (`setup_hooks: cursor`) gets them in its `sessionStart` hook, when a chat starts
  (a resumed chat already has them).
- Other agents have no way to receive them without editing their files, which panal never does;
  the tab lists them.

Changes apply to sessions started after saving.

## Past sessions

When an agent session ends (it exits, you stop it, or you discard it while suspended), panal keeps
it in `~/.panal/history/` (`<id>.json` and `<id>.txt`, readable only by you):

- its name, agent, model, project, folder, branches and when it started and ended;
- the prompts you typed, for agents with a readable transcript (Claude Code today);
- its terminal text as plain text (colors and cursor movement removed), the last 512 KB.

Before anything is written, what looks like a secret (API keys, tokens, private keys,
`password=…`) is replaced with `[redacted]`. Shells are not kept, nor are sessions a stopping
daemon suspends (they aren't over). `history.keep` (300) is how many are kept: the oldest go
first. `history.enabled: false` stops keeping new ones (the existing ones stay until you delete
them or the folder).

The clock in the sidebar (⌘⇧F; Linux: Ctrl+Shift+H) opens **Past sessions**: search as you type
(every word must appear in the session), in the current project or in all of them, read a session
with its prompts and text, **Resume** it or **Delete** it. **Resume** opens a new session of the
same agent in the same folder that continues its conversation (`claude --resume <id>`,
`codex resume <id>`); it's offered when the agent reported its conversation id and the project,
profile and folder still exist. From the CLI: [`panal history`](cli.md#panal-history---project-p--n-n-words).

## Browser for agents

Agents test web apps, open pages and take screenshots. Left alone they drive your own Chrome: they
move and resize its window, open tabs among yours and see your logins. panal gives each session
**its own Chrome** instead:

- its own profile in `~/.panal/browsers/<sessionId>/` (none of your tabs, cookies, extensions or
  logins), a fixed page size (1280×800) and its own DevTools port on `127.0.0.1`, from 9222;
- it **opens only when the agent needs it** and closes when the session exits; the profile is
  deleted when the session is removed (logins made there last as long as the session);
- **Claude Code** and **codex** sessions get it as browser tools: the
  [`chrome-devtools-mcp`](https://github.com/ChromeDevTools/chrome-devtools-mcp) MCP server (run
  with `npx`, pinned to one version, usage statistics and CrUX lookups off), added for that
  invocation only (`--mcp-config` / `-c mcp_servers…`): their own configuration is left alone. It
  launches the session's Chrome on its first tool call;
- every session gets `PANAL_BROWSER_URL` (`http://127.0.0.1:<port>`), and `panal browser`
  starts that Chrome and prints the URL, for any agent or tool (Playwright's `connectOverCDP`,
  Puppeteer's `connect({browserURL})`);
- the instructions handed to Claude Code and codex (see
  [Standing instructions](#standing-instructions-for-the-agents)) tell them to use it and never your
  Chrome;
- a 🌐 in the session's row shows its browser is open; click it to copy the URL, ⌥-click to close it;
- while it's open, the session's **Browser** tab shows it live: its tabs (the one the agent last opened or
  navigated is shown; click another to watch it), the page's address and a button to close it. It only
  shows: the agent drives the browser, your clicks don't reach the page.

It needs a Chrome-compatible browser (Google Chrome, Chromium, Microsoft Edge, Brave or
`browser.path`) and, for the browser tools, Node.js (`npx`); **Settings → General** shows what was
found. Turn it off with `browser.enabled: false`; by default it runs without a window (you watch it in the Browser tab); `browser.headless: false` also gives it its own window.

An agent that uses a browser extension (such as Claude in Chrome) still reaches your Chrome through
it: panal cannot stop that, it only gives the agent a better browser and tells it to use it.
