# Compatibility and versioning

panal follows [Semantic Versioning](https://semver.org/). While the version is 0.x a minor
release may still break things, and the [CHANGELOG](../CHANGELOG.md) says so when it does. From 1.0
on, the surfaces below only change in a backward-compatible way within a major version.

## What is stable

| Surface | What is promised |
| --- | --- |
| **CLI** | The commands and flags in [cli.md](cli.md) keep their names and meaning. New commands and flags may appear. The human-readable output (`ls`, `profiles`, `worktrees`…) may change: don't parse it. A non-zero exit status always means failure. |
| **Environment variables** | `PANAL_HOME`, `PANAL_NOTIFY`, `PANAL_SESSION` and `PANAL_SOCK`, as documented in [cli.md](cli.md#environment-variables), and what every session gets: `PANAL_PORT` ([worktrees.md](worktrees.md)) and `PANAL_BROWSER_URL` ([configuration.md](configuration.md#browser-for-agents)). |
| **Configuration files** | `profiles.yaml`, `settings.yaml`, a repository's `.panal.yaml` and the instruction topic files (their front matter), see [configuration.md](configuration.md) and [worktrees.md](worktrees.md): keys keep their meaning, and new keys are optional with a default. Each file says its format version (below). |
| **Hooks** | The command panal installs in an agent's hooks (`<panal> notify --sock … --from-hook …`, see [attention-and-activity.md](attention-and-activity.md)) keeps working, so hooks installed by an older version don't need reinstalling. |
| **MCP tools** | The tools of panal's MCP server (`panal mcp`, see [mcp.md](mcp.md)) keep their names and arguments, and the fields of their structured results keep their meaning. New tools, arguments and fields may appear. The text they return is for the model and may change. |
| **The old name** | What lets an agenterm install keep working (moving `~/.agenterm` and `~/.config/agenterm`, the `AGENTERM_*` variables, an `agenterm` command or link, `.agenterm.yaml`) stays for all of 1.x. |
| **Daemon protocol** | Versioned (below). It's how the app, the CLI and the daemon talk; it isn't a public API, but versions of panal that disagree detect it and say what to do. |

Not covered: `daemon.log` (its content changes freely), the runtime files `restart.json`, `d.pid`
and `scrollback/`, the `.bak` and `.corrupt-*` copies, and anything under `internal/` in the code.

## File formats

Every file panal writes says which format it uses:

| File | Where | How the version is written | Current |
| --- | --- | --- | --- |
| `settings.yaml` | `~/.config/panal/` | top-level `version:` key | 1 |
| `profiles.yaml` | `~/.config/panal/` | `version:` next to the `profiles:` list | 1 |
| `projects.json` | `~/.config/panal/` | `{"version": 1, "projects": [...]}` | 1 |
| `worktrees.json` | `~/.panal/` | `{"version": 1, "worktrees": [...]}` | 1 |
| `sessions.json` | `~/.panal/` | `{"version": 1, "sessions": [...]}` | 1 |
| `messages.json` | `~/.panal/` | `{"version": 1, "messages": [...]}` (since 0.6.0) | 1 |
| `trust.json` | `~/.config/panal/` | `{"version": 1, "trusted": [...]}` | 1 |
| `.panal.yaml` | a repository's root | top-level `version:` key | 1 |
| `history/<id>.json` | `~/.panal/` | a `"version"` field in each entry (entries saved before it was added have none: format 1) | 1 |

- **Files from before 0.4.0** have no version: they are format 1 and are read as they are
  (`profiles.yaml` as a bare list, the JSON files as bare arrays). panal adds the version the
  next time it saves the file. You don't need to change anything by hand.
- **A newer format than this build understands** means the file was written by a newer panal
  (for example, after going back to an older version). panal refuses it with an error that says
  so (`… was written by a newer panal (format 2; this build reads up to 1): update panal`) and
  **never overwrites it**, so no data is lost. `settings.yaml` is the exception on reading: what this
  build understands is still applied as far as it can be read, but it isn't saved over either.
- A format version only goes up when an older panal would misread the file. Adding an optional
  key doesn't change it. An older panal ignores keys it doesn't know; `settings.yaml` keeps them
  when it's saved, but the other files are rewritten from what that version understands, so it drops
  them on its next save (the newer version then uses their defaults).
- A version that isn't a positive whole number is treated like a newer one: the file is refused and left alone.

### Going back to an older version

0.3.x and older don't know the versioned formats: once 0.4.0 has saved a file, an older panal
can't read it. Before starting the older version, turn each file back into its old shape (the
content stays the same):

- `profiles.yaml`: keep only the entries under `profiles:`, as a top-level list (remove `version:`
  and `profiles:`, and the two spaces of indentation).
- `projects.json`, `worktrees.json`, `sessions.json`: keep only the array inside `"projects"`,
  `"worktrees"` or `"sessions"`.
- `settings.yaml` needs nothing: older versions ignore the `version` key.

The `.bak` copies don't help here: after a couple of saves they are in the new format too. If you
already started 0.3.x without converting `sessions.json`, it was moved aside as
`sessions.json.corrupt-<time>` and its sessions weren't restored; convert that file and rename it
back to `sessions.json` (with the daemon stopped) to get them back.

## Daemon protocol

The app and the CLI talk to the daemon over its socket with newline-delimited JSON (see
[architecture.md](architecture.md)). `daemon.ping` reports the daemon's version and its protocol
version (`protocol`, currently 1; daemons before 0.4.0 don't send it and speak version 1).

- Adding methods, optional fields or event types keeps the protocol version. Removing or renaming
  them, or changing what they mean or the binary frames of `session.attach`, bumps it.
- After an update the window runs the new build while the daemon, which outlives the window, may
  still be the old one: the app restarts it (suspending and resuming the sessions) on its own when
  no agent is in a turn, and otherwise says so and offers **Restart now**.
- If the daemon is **newer** than the app (a newer protocol), restarting it from the app would
  downgrade it, so the app asks you to update panal instead.
- A command the daemon doesn't know (an older daemon) fails with a clear message (code
  `unknown_method`): restart the daemon, from the app or with `panal stop`.
