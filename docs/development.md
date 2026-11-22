# Development

**English** · [Español](es/development.md)

Requirements: those in [README.md](../README.md#build-from-source) (Go, Node 24+ with npm and
the Wails v2 CLI). The rules for opening a pull request are in [CONTRIBUTING.md](../CONTRIBUTING.md).

## App in development mode

```sh
cd app
wails dev     # hot reload (frontend on Vite); also serves http://localhost:34115
wails build   # -> app/build/bin/panal.app
```

Frontend only:

```sh
cd app/frontend
npm ci
npm run check   # svelte-check (types)
npm test        # Vitest unit tests (src/**/*.test.ts)
npm run build   # vite build
```

## Tests

```sh
gofmt -l .                       # must print nothing
go vet ./... && go test -race ./...
```

This is what CI runs (plus the frontend's `npm run check`, `npm test` and `npm run build`).

- One integration test brings up the daemon in a temporary `PANAL_HOME` and exercises spawn,
  attach/detach/reattach with scrollback, notify, events, kill and shutdown; another launches a
  shell in worktree mode on a temporary repo, makes changes by typing into it and checks
  `session.diff`, that `kill` keeps the worktree, the orphan after restarting the daemon, reopening
  it, the merge and `worktree.remove` (refusal and force).
- `internal/daemon/persist_test.go` covers persistence: a shell session writes output, the daemon
  stops (`stop --force` and cancellation), a new daemon on the same `PANAL_HOME` lists it as
  `suspended` with its scrollback replayed, resumes it (same folder, "session restored" line) and
  discards another; the `always`/`never` policies; a fake `setup_hooks: claude` script that
  checks `--resume <session_id>` comes after the `--settings` injection; failed resumes and a
  corrupt `sessions.json`. No real agent is ever launched.
- The `internal/gitx` tests use real repos in temporary directories with isolated git configuration
  (`GIT_CONFIG_NOSYSTEM=1`, empty `GIT_CONFIG_GLOBAL`, temporary `HOME`).
- `go test` never posts real notifications: test daemons get a no-op or recording notifier (a
  `daemon.Config` without one posts nothing), the app tests use a fake desktop, and the `app`,
  `internal/daemon` and `internal/daemoncmd` tests set `PANAL_NOTIFY=off` in `TestMain`. Keep it
  that way in new tests.

## Testing without touching your installation

Use your own `PANAL_HOME`, and set `PANAL_NOTIFY=off`. That's the rule for **any** test or
demo instance (daemon, app or CLI that may auto-start a daemon):

- `PANAL_HOME` must be a **short** path (`/tmp/…`): on macOS a unix socket path is limited to
  about 104 bytes and the temporary folders under `/var/folders/…` go over it.
- `PANAL_NOTIFY=off` disables every desktop notification of that app and daemon (native and the
  `osascript` fallback), whatever its `settings.yaml` says, so test sessions don't pop up
  notifications (or Script Editor) on your desktop. The daemon inherits it from whoever starts it.

```sh
export PANAL_HOME=/tmp/agt PANAL_NOTIFY=off
go run ./cmd/panal project add .
go run ./cmd/panal spawn panal shell
go run ./cmd/panal ls
go run ./cmd/panal stop --force      # stops only the /tmp/agt daemon

cd app && PANAL_HOME=/tmp/agt PANAL_NOTIFY=off wails dev   # the app against that same daemon
```

Since `PANAL_HOME` takes precedence over `PANAL_SOCK`, this is also safe inside a panal
session: it doesn't touch the daemon hosting the session.

## Wails bindings

The TypeScript bindings for the methods in `app/app.go` live in `app/frontend/wailsjs/` and are
checked in. `wails dev` and `wails build` regenerate them; if you change an exported method's
signature and only want the bindings:

```sh
cd app && wails generate module
```

## Releasing

The version has a single source of truth: the git tag `vX.Y.Z`. Release builds inject it with
`-ldflags -X` into `internal/version` (what `panal version` and the app's About panel show) and
write it into the bundle's `Info.plist` (`CFBundleShortVersionString` and `CFBundleVersion`).
`productVersion` in `app/wails.json` is only what plain `wails build` uses; builds without ldflags
report `dev`.

1. Move the `[Unreleased]` entries of [CHANGELOG.md](../CHANGELOG.md) into a `## [X.Y.Z] - date`
   section (and update the links at the bottom), and merge that to `main`.
2. Tag `main` and push the tag:

   ```sh
   scripts/tag-release.sh 0.1.0      # -n: check only
   ```

   It tags `origin/main` (not your checkout) once its CHANGELOG has the `## [0.1.0]` section and
   CI passed on that commit (it waits while CI runs), then pushes `v0.1.0`. By hand, that is
   `git tag -a v0.1.0 -m "panal 0.1.0" origin/main && git push origin v0.1.0`.

3. `.github/workflows/release.yml` runs on the tag (macOS runner) and:
   - builds everything with `scripts/release-local.sh <version>` (the same script you run locally):
     the universal app (`wails build -platform darwin/universal`), the universal CLI (cgo, `lipo`)
     bundled at `panal.app/Contents/Helpers/panal`, and the Linux CLI (`CGO_ENABLED=0`);
   - embeds Sparkle (see [In-app updates](#in-app-updates-sparkle)) and signs it inside-out
     before the rest;
   - signs the CLI and then the app with the Developer ID, hardened runtime, a secure timestamp and
     `app/build/darwin/entitlements.plist`, if the certificate secrets exist; otherwise it keeps an
     ad-hoc signature, warns, and the release notes say it's an unsigned build;
   - notarizes the zip and then the dmg with `notarytool` and staples both, if the App Store Connect
     secrets exist too; a rejected submission fails the job and prints the `notarytool log`;
   - packages `panal-<v>-macos-universal.{zip,dmg}`, `panal-<v>-darwin-universal.tar.gz`,
     `panal-<v>-linux-{amd64,arm64}.tar.gz` and `checksums.txt`, and uploads them to the run;
   - if `SPARKLE_PRIVATE_ED_KEY` exists, EdDSA-signs the (stapled) zip and writes `appcast.xml`;
   - creates the GitHub release `vX.Y.Z` with the CHANGELOG section as notes (or generated notes if
     there's none) and every artifact attached, `appcast.xml` included when the build is notarized.
     Once `SPARKLE_PUBLIC_ED_KEY` is set, a release without `appcast.xml` isn't marked *Latest* (the
     feed URL points at the latest release). A pre-release (`X.Y.Z-suffix`) is published as a GitHub pre-release: never *Latest*,
     never offered in-app, and the Homebrew cask isn't updated;
   - if `HOMEBREW_TAP_TOKEN` exists, renders `packaging/homebrew/panal.rb.tmpl` with the version
     and the zip's sha256 (`scripts/render-cask.sh`) and pushes it to `Casks/panal.rb` in
     [elionaz/homebrew-tap](https://github.com/elionaz/homebrew-tap) as "panal X.Y.Z".

### Secrets

Set them in the repo's *Settings → Secrets and variables → Actions*, or with `gh secret set`. All of
them are optional: without them the release is still published, unsigned, and the cask isn't
updated.

| secret | used for |
|---|---|
| `MACOS_CERT_P12_BASE64` | the Developer ID Application certificate and its private key (`.p12`), base64 |
| `MACOS_CERT_PASSWORD` | the `.p12`'s password |
| `MACOS_SIGN_IDENTITY` | optional: the identity name, e.g. `Developer ID Application: Jane Doe (TEAMID)`; by default the first Developer ID Application identity in the `.p12` |
| `APPLE_API_KEY_ID` | App Store Connect API key id (notarization) |
| `APPLE_API_ISSUER_ID` | App Store Connect issuer id (notarization) |
| `APPLE_API_KEY_P8_BASE64` | the API key's `.p8` file, base64 (notarization) |
| `HOMEBREW_TAP_TOKEN` | a GitHub token that can push to `elionaz/homebrew-tap` |
| `SPARKLE_PRIVATE_ED_KEY` | Sparkle's EdDSA private key, as `generate_keys -x` exports it: signs the update zip and enables `appcast.xml` |
| `SPARKLE_PUBLIC_ED_KEY` | its public key (`SUPublicEDKey`); a repository **variable** (it isn't secret), or a secret |

Without `SPARKLE_PUBLIC_ED_KEY` the app is built with its updater off; without
`SPARKLE_PRIVATE_ED_KEY` there's no `appcast.xml`, so installed apps aren't offered the release.
Both cases only warn.

**Developer ID Application certificate** (needs the Account Holder role in the Apple Developer
program):

1. In *Keychain Access → Certificate Assistant → Request a Certificate From a Certificate
   Authority…*, enter your email, choose "Saved to disk" and save the `.certSigningRequest`.
2. In [developer.apple.com → Certificates](https://developer.apple.com/account/resources/certificates/list),
   click **+**, choose **Developer ID Application** (the G2 Sub-CA), upload the request and download
   the `.cer`. Double-click it to add it to your login keychain.
3. In Keychain Access → *My Certificates*, right-click "Developer ID Application: … (TEAMID)" (it
   must show its private key) → **Export…** → `.p12` with a strong password.
4. Store it:

   ```sh
   base64 -i DeveloperID.p12 | gh secret set MACOS_CERT_P12_BASE64
   gh secret set MACOS_CERT_PASSWORD          # paste the password
   security find-identity -v -p codesigning   # the exact name, for MACOS_SIGN_IDENTITY
   ```

   Then delete the exported `.p12`.

**App Store Connect API key** (notarization):

1. In [App Store Connect](https://appstoreconnect.apple.com/access/integrations/api) → *Users and
   Access → Integrations → App Store Connect API → Team Keys*, generate a key with the
   **Developer** role.
2. Download `AuthKey_<KEYID>.p8` (Apple only lets you download it once) and note the **Key ID** and
   the **Issuer ID** shown above the list.
3. Store them:

   ```sh
   base64 -i AuthKey_XXXXXXXXXX.p8 | gh secret set APPLE_API_KEY_P8_BASE64
   gh secret set APPLE_API_KEY_ID --body XXXXXXXXXX
   gh secret set APPLE_API_ISSUER_ID --body 00000000-0000-0000-0000-000000000000
   ```

**`HOMEBREW_TAP_TOKEN`**: a
[fine-grained personal access token](https://github.com/settings/personal-access-tokens/new) with
*Repository access → Only select repositories → elionaz/homebrew-tap* and the permission
*Contents: Read and write* (nothing else). Store it with `gh secret set HOMEBREW_TAP_TOKEN` and
renew it before it expires.

### Dry runs

Locally (macOS, with the requirements in the README):

```sh
scripts/release-local.sh 0.1.0        # -> dist/ (gitignored), ad-hoc signed
codesign --verify --deep --strict --verbose=2 dist/work/panal.app
lipo -info dist/work/panal.app/Contents/MacOS/panal
scripts/render-cask.sh 0.1.0 "$(shasum -a 256 dist/panal-0.1.0-macos-universal.zip | cut -d' ' -f1)"
```

To try the signing path, pass an identity from your keychain: `SIGN_IDENTITY="Developer ID
Application: …" scripts/release-local.sh 0.1.0`. Notarization also needs `NOTARY_KEY_PATH` (the
`.p8`), `NOTARY_KEY_ID` and `NOTARY_ISSUER_ID`.

On GitHub: *Actions → Release → Run workflow* with a version. It builds, signs and notarizes (if the
secrets exist) and uploads the artifacts to the run (`appcast.xml` included), but doesn't create a
release or touch the tap.

To try the appcast locally without your real key, use a throwaway ed25519 pair (OpenSSL 3, no
keychain involved):

```sh
k=$(mktemp -d)
openssl genpkey -algorithm ed25519 -out "$k/key.pem"
export SPARKLE_PRIVATE_ED_KEY=$(openssl pkey -in "$k/key.pem" -outform DER | tail -c 32 | base64)
export SPARKLE_PUBLIC_ED_KEY=$(openssl pkey -in "$k/key.pem" -pubout -outform DER | tail -c 32 | base64)
scripts/release-local.sh 0.2.0-dev
xmllint --noout dist/appcast.xml
```

### In-app updates (Sparkle)

The app embeds [Sparkle 2](https://sparkle-project.org): `scripts/fetch-sparkle.sh` downloads the
pinned release (`SPARKLE_VERSION`), checks it against the SHA-256 GitHub publishes for it and
caches it in `.cache/sparkle/` (gitignored). `release-local.sh` then:

- builds the app with `-tags sparkle` (`app/updater_sparkle_darwin.{go,m}`; every other build,
  `wails dev` and tests included, gets `app/updater_stub.go`), weak-linking `Sparkle.framework`
  through `@rpath` = `Contents/Frameworks`;
- copies the framework to `panal.app/Contents/Frameworks/` and signs, in this order and without
  `--deep`, as Sparkle documents for Developer ID apps: `XPCServices/Installer.xpc`,
  `XPCServices/Downloader.xpc` (`--preserve-metadata=entitlements`), `Autoupdate`, `Updater.app`,
  the framework; then the CLI and the app as before;
- sets `SUPublicEDKey` in `Info.plist` (the template has `SUFeedURL` =
  `https://github.com/elionaz/panal/releases/latest/download/appcast.xml`, daily automatic checks
  and automatic updates allowed), or removes it when `SPARKLE_PUBLIC_ED_KEY` is empty: the app then
  keeps the updater off;
- after notarizing and stapling the zip, signs it with `bin/sign_update --ed-key-file` (the key goes
  through a 0600 temp file, never the command line or the log), verifies the signature, checks it
  against `SPARKLE_PUBLIC_ED_KEY` with OpenSSL and writes `dist/appcast.xml`
  (`scripts/render-appcast.sh`: a single item, the release's zip, `minimumSystemVersion` 13.0 and
  the CHANGELOG section as release notes).

The release uploads `appcast.xml` as an asset, so `releases/latest/download/appcast.xml` always
points at the newest release. Unnotarized builds don't publish it: Sparkle would install an app
Gatekeeper then refuses to open.

The app turns its updater off, and says why in **Settings → General → Updates** and in **Check for
Updates…**, when it's a development build and when it has no `SUPublicEDKey`. Homebrew installs
update through Sparkle like any other, and the cask declares `auto_updates true` (since 0.4.3), so
plain `brew upgrade` skips it; `--greedy` or naming the cask still upgrades it. It came one release
after Sparkle was turned on for Homebrew because brew reads it from the tap's cask, not from the
installed app: installs from 0.4.1 and earlier (Sparkle off) needed a release to move to with plain
`brew upgrade`. brew's record (`Caskroom/panal/<version>`) stays at the version brew installed
after Sparkle updates the app; that is expected for `auto_updates` casks, and the CLI symlink
points at a fixed path.

**The update banner.** Sparkle is created with an updater delegate and a standard user driver
delegate (`AgtSparkleDelegate` in `updater_sparkle_darwin.m`) that report to Go
(`agtSparkleUpdate` → Wails event `updates.available` `{version, ready}`, also in
`UpdaterState().available`): `updater:didFindValidUpdate:` records the version,
`updaterDidNotFindUpdate:` and a *Skip this version* choice (`updater:userDidMakeChoice:…`) clear
it, and `updater:willInstallUpdateOnQuit:immediateInstallationBlock:` marks it ready. That one
returns `NO`: `YES` would make the immediate installation block usable but stalls Sparkle's update
cycle until the app quits (no later or critical release found, no impatient reminders). With `NO`
the cycle goes on and Sparkle still installs the update on quit. It implements
[gentle reminders](https://sparkle-project.org/documentation/gentle-reminders): a scheduled update
shows Sparkle's window only when Sparkle would put it in immediate focus (just launched, or the Mac
was idle); otherwise the banner is the reminder. `InstallUpdateNow` (both **Update** and **Restart
to update**) calls `checkForUpdates`, which brings the window with the update found — for a
downloaded one, its *Install and Relaunch* prompt (the daemon is untouched). While Sparkle is still
downloading in the background `canCheckForUpdates` is `NO` and that call would do nothing, so
`InstallUpdateNow` returns `false` and the button shows *Downloading…* until the update is ready.
"Later" is remembered per version and stage (available / ready) in
the webview's `localStorage`. The banner yields to the reconnecting and daemon-restart ones.

**Generating the key pair** (once, on your Mac; the private key never goes into the repo):

```sh
SPARKLE_DIR=$(scripts/fetch-sparkle.sh)
"$SPARKLE_DIR/bin/generate_keys" --account panal          # creates it in your login keychain, prints the public key
"$SPARKLE_DIR/bin/generate_keys" --account panal -x sparkle-private.txt
gh secret set SPARKLE_PRIVATE_ED_KEY < sparkle-private.txt
gh variable set SPARKLE_PUBLIC_ED_KEY --body "$("$SPARKLE_DIR/bin/generate_keys" --account panal -p)"
rm sparkle-private.txt
```

Keep a backup of the private key (the keychain item, or the exported text in a password manager):
if it's lost, installed apps can't verify updates any more and users have to download the next
version by hand.

**Rotating the key**: generate a new pair (another `--account`), then publish one release signed
with the **old** private key that ships the **new** public key: keep `SPARKLE_PRIVATE_ED_KEY` as
is, set the variable `SPARKLE_PUBLIC_ED_KEY` to the new key and `SPARKLE_KEY_ROTATION=1` (a
repository variable; it skips the check that both keys are a pair). Once that release is out,
replace `SPARKLE_PRIVATE_ED_KEY` with the new private key and delete `SPARKLE_KEY_ROTATION`. If the
old key leaked, do the same right away; if it's lost, there's no in-app path: announce that the
next version has to be installed by hand.

**The daemon after an update.** Sparkle replaces and relaunches the app, but the daemon (started
from the old executable) keeps running, by design, with the sessions. On every (re)connection the
app compares `daemon.ping`'s `version` (absent in daemons before 0.2.0) with its own and, if the
daemon is older, restarts it on its own when no session is `running` (once per app run, so a
restart that leaves it old doesn't loop; `shouldAutoRestart` in `app/frontend/src/lib/daemon.ts`).
Otherwise it shows a banner with **Restart now** (with a confirmation when agents are working).
`RestartDaemon`:

1. writes the ids of the live sessions to `restart.json` in the runtime dir;
2. stops the daemon with force (live sessions are recorded as suspended) and waits for its
   **process** to exit — its pid file goes away last thing — not just for the socket to stop
   answering: the old daemon is still killing its agents then, and daemons before 0.2.0 remove the
   socket path as their very last step, which would orphan a new daemon already listening there.
   A pid reused by another program counts as gone; a daemon still there after 20 s is SIGKILLed
   (its sessions are already recorded as suspended);
3. starts the new daemon, which restores the sessions in `restart.json` as suspended whatever
   `sessions.restore` says (and doesn't auto-resume them under `always`), and deletes the file;
4. resumes exactly those sessions. Sessions that were already suspended stay so.

Development builds (`dev`) never show the banner, nor does a daemon newer than the app.

## Icon

The sources are in `assets/icon/`:

- `panal-icon.svg`: the square, full-bleed icon;
- `panal-icon-rounded.svg`: with rounded corners (the one in the README);
- `panal-icon-macos.svg`: with the macOS icon template's margin, which is the one the app uses.

Wails generates the `.app` icon from `app/build/appicon.png` (1024×1024). To regenerate it with
Inkscape 1.x:

```sh
inkscape assets/icon/panal-icon-macos.svg \
  --export-type=png --export-filename=app/build/appicon.png -w 1024 -h 1024
```

On macOS the Inkscape CLI isn't on the `PATH`; it's at
`/Applications/Inkscape.app/Contents/MacOS/inkscape`. Use solid colors on the edges (not
`stroke-opacity`): semi-transparent strokes add up where they overlap.

## Known limitations

- **macOS first.** The app is mostly tested on macOS (Apple Silicon); the Linux app is experimental
  (below). The transport is a unix socket; Windows isn't supported.
- **Upgrading doesn't restart the daemon**: after installing a new version, a daemon that was
  already running keeps the old code until `panal stop` (or a computer restart).
- The **Linux app** (AppImage, .deb) is experimental: CI builds, tests and packages it on Ubuntu
  24.04 (`scripts/package-linux.sh`), but it's lightly tested on real desktops. Every change runs
  the daemon and CLI tests on Ubuntu (ci.yml `linux`, no GTK); the app's tests and packaging
  (`linux-app.yml`, with GTK/WebKitGTK) run on main and on pull requests that touch `app/`,
  `packaging/`, the workflows or the dependencies.
- **A daemon restart (or a computer restart) kills the processes.** Sessions come back as
  `suspended`, with their scrollback, and are respawned on resume: Claude Code and codex continue
  their conversation by id, opencode and Cursor CLI with `--continue` and Hermes with `-c` (each project's *last*
  session, which may not be the same one if several run in one folder), other agents only with
  `resume_args`. A shell comes back in its starting folder, without whatever was running in it. codex only reports its
  thread id after the first turn, so a codex session that never finished a turn starts fresh.
- **Scrollback persistence costs disk writes**: the whole ring (up to 8 MiB) is rewritten at most
  every 2 s per session while it produces output.
- **Notifications with the app closed**: if the daemon isn't inside the `.app` (a CLI installed
  outside the bundle started it) or the bundle has no permission, notifications are not shown
  (`daemon.log` says why). With `notify.fallback: osascript` they are sent through `osascript`
  instead: they show up as *Script Editor*, and clicking one opens Script Editor, not panal.
- **codex** doesn't report when it starts working: "working" is decided by the heuristic and there's
  no "stalled?" warning.
- **"Stalled?"** only exists in sessions with hooks (Claude Code), and since Claude's spinner
  redraws every second, it mostly catches complete hangs, not slow tools.
- **Activity line in TUIs** that redraw the screen (opencode, codex…): it's the last line they drew,
  which isn't always the most informative.
- **Kiro CLI**: the binary name (`kiro-cli chat`) comes from its documentation and hasn't been
  verified against a real installation; you can fix it with `cmd:` in `profiles.yaml`.
- **Hermes Agent** launches as `hermes --cli` and only from the **+** menu when the binary is
  installed and the profile is enabled. The approval line `[o]nce | [s]ession` marks the session
  as waiting. `args: ["--tui"]` or `args: []` replaces that default; `resume_args: []` skips the
  `-c` reattach; `enabled: false` hides it. Closing the terminal detaches from Hermes's gateway
  (the turn keeps running until `/exit` or Ctrl+D). panal does not write `~/.hermes`.
- **`profiles.yaml` loses its comments** when the app saves it (it's rewritten in canonical form).
  `settings.yaml` keeps them.
- Merging a worktree **doesn't stash**: the main folder must be clean and on the base branch.
