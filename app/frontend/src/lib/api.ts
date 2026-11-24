// Thin typed layer over the generated Wails bindings.
import * as App from '../../wailsjs/go/main/App'
import type { ProjectPatch } from './badge'
import type { forge, gitx, history, main, profile, project, protocol, settings } from '../../wailsjs/go/models'
import { hasKey, t, type Params } from './i18n.svelte'
import type { Message } from './messages'

// Plain data (the generated classes carry a convertValues helper we never use).
export type Project = Omit<project.Project, 'convertValues'>
export type Profile = profile.Profile
/** A profiles.yaml entry as sent to profile.save (desired values). */
export type ProfileEntry = profile.Entry
export type CatalogInfo = Omit<protocol.CatalogInfo, 'convertValues'>
export type Settings = Omit<settings.Settings, 'convertValues'>
/** The user's standing instruction topics (instructions.list). */
export type Instructions = Omit<protocol.Instructions, 'convertValues'>
export type InstructionTopic = protocol.InstructionTopic
export type BrowserStatus = protocol.BrowserStatus
export type HandoffNote = protocol.HandoffNote
export type Session = Omit<protocol.SessionInfo, 'convertValues'>
/** suspended: restored from a previous daemon run, no process (resume or discard it). */
export type SessionState = 'running' | 'waiting_input' | 'idle' | 'exited' | 'suspended'
/** Mirrors protocol.SessionBatch (session.resume / discard). */
export type SessionBatch = Omit<protocol.SessionBatch, 'convertValues'>
export type AttentionKind = 'permission' | 'question' | 'done' | 'error'
export type SpawnMode = 'same' | 'worktree'
export type SpawnOptions = main.SpawnOptions
export type GitInfo = Omit<protocol.GitInfo, 'convertValues'>
export type DiffFile = gitx.DiffFile
export type Commit = Omit<gitx.Commit, 'convertValues'>
export type DiffResult = Omit<protocol.DiffResult, 'convertValues'>
export type WorktreeInfo = Omit<protocol.WorktreeInfo, 'convertValues'>
export type ProjectGit = protocol.ProjectGit
export type LossInfo = Omit<gitx.LossError, 'convertValues'>
export type HistoryEntry = Omit<history.Entry, 'convertValues'>
export type HistorySnippet = history.Snippet
export type HistoryHit = Omit<history.Hit, 'convertValues'>
export type HistoryDetail = Omit<protocol.HistoryDetail, 'convertValues'>
export type RemoveMergedResult = Omit<protocol.WorktreeRemoveMergedResult, 'convertValues'>
export type MergeResult = gitx.MergeResult
/** url '' = a host we cannot build a pull request page for (pushed only). */
export type PRResult = gitx.PRResult
/** The pull request of a branch on GitHub / GitLab (forge.status). */
export type ForgeStatus = Omit<protocol.ForgeStatus, 'convertValues'>
export type ForgePR = Omit<forge.PR, 'convertValues'>
export type ForgeComment = Omit<forge.Comment, 'convertValues'>
export type ForgeList = Omit<protocol.ForgeList, 'convertValues'>
export type ForgeListItem = Omit<protocol.ForgeListItem, 'convertValues'>
/** In-app updates (Sparkle). reason: '' (enabled) | unsupported | nobundle | nokey | error | starting. */
export type UpdaterState = Omit<main.UpdaterState, 'convertValues'>
/** A newer release: ready = downloaded (installs on quit or via installUpdateNow). */
export type UpdateInfo = main.UpdateInfo
/** daemonVersion '' = a daemon older than 0.2.0. */
export type DaemonStatus = main.DaemonStatus
export type OnboardingInfo = main.OnboardingInfo

/** Mirrors protocol.Progress (terminal OSC 9;4 progress). */
export interface Progress {
  state: 'normal' | 'error' | 'indeterminate' | 'paused'
  value: number
}

/** Mirrors protocol.Live: activity line, timers, stall, progress. */
export interface Live {
  /** English for hook activity (render activityKind/Target instead);
   * verbatim for output lines. */
  activity?: string
  /** thinking | edit | read | bash | search | web | subagent | todo | mcp | tool */
  activityKind?: string
  activityTarget?: string
  activityAt?: string
  activitySource?: 'hook' | 'output'
  turnStartedAt?: string
  turnEndedAt?: string
  lastOutputAt?: string
  lastEventAt?: string
  stalled?: boolean
  stalledAt?: string
  progress?: Progress
}

/** Mirrors protocol.Event (Go). */
export interface DaemonEvent extends Live {
  type:
    | 'session.spawned'
    | 'session.state'
    | 'session.removed'
    | 'session.attention'
    | 'session.activity'
    | 'session.git'
    | 'session.renamed'
    | 'worktree.changed'
    | 'project.changed'
    | 'session.reordered'
    | 'message.changed'
    | 'profile.changed'
    | 'session.model'
    | 'session.browser'
    | 'history.changed'
    | 'message.sent'
  sessionId: string
  /** The session's display name (session.renamed: the new one; absent: reset). */
  sessionName?: string
  state?: SessionState
  prevState?: string
  source?: string
  exitCode?: number
  at: string
  attentionKind?: AttentionKind
  attentionMessage?: string
  attentionAt?: string
  unread?: boolean
  /** A new attention was raised (the backend already decided on notifying). */
  raised?: boolean
  /** session.activity: the session just became stalled. */
  stallRaised?: boolean
  /** session.git: the new change summary (absent: no longer a git folder). */
  git?: GitInfo
  /** worktree.changed, message.changed: the project. */
  projectId?: string
  /** message.changed: messages waiting for sessionId. */
  msgPending?: number
  /** message.changed: why they wait (SessionInfo.msgHold). */
  msgHold?: string
  /** message.sent: who wrote to whom (session ids or "user"), their names,
   * the start of the text, and whether it waits for the user (manual). */
  msgId?: number
  msgFrom?: string
  msgFromName?: string
  msgTo?: string
  msgToName?: string
  msgExcerpt?: string
  msgHeld?: boolean
  /** session.model: the agent's model id. */
  model?: string
  /** session.state to exited: panal ended it (no failure). */
  stopped?: boolean
  /** session.browser: the session's own Chrome opened (its DevTools URL) or closed (absent). */
  browserUrl?: string
}

/** What a session is called: its given name, else its profile. */
export function sessionTitle(s: Pick<Session, 'name' | 'profileId'>): string {
  return s.name || s.profileId
}

/** A session needs the user and they have not looked at it yet. */
export function needsAttention(s: Session): boolean {
  return !!s.unread && !!s.attentionKind && s.state !== 'exited'
}

export function attentionLabel(kind: string | undefined): string {
  switch (kind) {
    case 'permission':
    case 'question':
    case 'done':
    case 'error':
      return t(`attention.${kind}`)
    default:
      return ''
  }
}

export const api = {
  listProjects: (): Promise<Project[]> => App.ListProjects(),
  /** Opens the native folder picker; resolves null if cancelled. */
  addProject: async (): Promise<Project | null> => (await App.AddProject()) ?? null,
  removeProject: (id: string) => App.RemoveProject(id),
  reorderProjects: (ids: string[]): Promise<Project[]> => App.ReorderProjects(ids),
  /** Renames a project or changes its badge: fields left out stay, an empty
   * color or icon goes back to automatic. */
  updateProject: (id: string, patch: ProjectPatch): Promise<Project> =>
    App.UpdateProject({ id, ...patch }),
  reorderSessions: (projectId: string, ids: string[]): Promise<Session[]> => App.ReorderSessions(projectId, ids),
  /** Every effective profile (hidden ones too), in menu order. */
  listProfiles: (): Promise<Profile[]> => App.ListProfiles(),
  profileCatalog: (): Promise<CatalogInfo[]> => App.ProfileCatalog(),
  saveProfile: (e: ProfileEntry): Promise<Profile[]> => App.SaveProfile(e),
  deleteProfile: (id: string): Promise<Profile[]> => App.DeleteProfile(id),
  reorderProfiles: (ids: string[]): Promise<Profile[]> => App.ReorderProfiles(ids),
  rescanProfiles: (): Promise<Profile[]> => App.RescanProfiles(),
  getSettings: (): Promise<Settings> => App.GetSettings(),
  /** The window gained or lost the focus (Linux; macOS asks NSApp). */
  windowFocus: (focused: boolean): Promise<void> => App.WindowFocus(focused),
  /** The drafted summary for continuing a session's work with another agent. */
  handoffNote: (sessionId: string): Promise<HandoffNote> => App.HandoffNote(sessionId),
  /** Starts profileId with note where sessionId worked (or in a new worktree); closes it unless keep. */
  handoff: (
    sessionId: string,
    to: { profileId: string; where: SpawnMode } | { targetId: string },
    note: string,
    keep: boolean,
    cols: number,
    rows: number,
  ): Promise<Session> =>
    'targetId' in to
      ? App.Handoff(sessionId, '', to.targetId, '', note, keep, cols, rows)
      : App.Handoff(sessionId, to.profileId, '', to.where, note, keep, cols, rows),
  /** Whether sessions get their own browser, and the Chrome and npx found. */
  browserStatus: (): Promise<BrowserStatus> => App.BrowserStatus(),
  closeBrowser: (sessionId: string): Promise<void> => App.CloseBrowser(sessionId),
  /** The Browser tab: a live image of the session's open browser (events
   * browser:pages:<id>, browser:frame:<id>, browser:end:<id>), one session at a time. */
  watchBrowser: (sessionId: string): Promise<void> => App.WatchBrowser(sessionId),
  pickBrowserPage: (sessionId: string, pageId: string): Promise<void> => App.PickBrowserPage(sessionId, pageId),
  unwatchBrowser: (sessionId: string): Promise<void> => App.UnwatchBrowser(sessionId),
  /** The instruction topics; with projectId, composed is what its sessions receive. */
  instructions: (projectId: string): Promise<Instructions> => App.Instructions(projectId),
  /** Saves a topic (new when oldSlug is ''). */
  saveInstruction: (t: InstructionTopic, oldSlug: string, projectId: string): Promise<Instructions> =>
    App.SaveInstruction(t, oldSlug, projectId),
  deleteInstruction: (slug: string, projectId: string): Promise<Instructions> => App.DeleteInstruction(slug, projectId),
  reorderInstructions: (slugs: string[], projectId: string): Promise<Instructions> =>
    App.ReorderInstructions(slugs, projectId),
  /** Fields left out keep their saved values. */
  saveSettings: (s: Partial<Settings>): Promise<Settings> => App.SaveSettings(s as main.SettingsPatch),
  listSessions: (): Promise<Session[]> => App.ListSessions(),
  spawn: (projectId: string, profileId: string, cols: number, rows: number, opts?: Partial<SpawnOptions>) =>
    App.SpawnIn(projectId, profileId, cols, rows, {
      mode: opts?.mode ?? '',
      branch: opts?.branch ?? '',
      base: opts?.base ?? '',
      worktreeId: opts?.worktreeId ?? '',
      trustSetup: opts?.trustSetup ?? false,
      skipSetup: opts?.skipSetup ?? false,
    }),
  /** Installs a catalog agent in a new terminal session of a project. */
  installAgent: (profileId: string, projectId: string, cols: number, rows: number): Promise<Session> =>
    App.InstallAgent(profileId, projectId, cols, rows),
  /** Copies files from the main folder into a worktree; save also writes
   * them into a new .panal.yaml. */
  copyIntoWorktree: (id: string, files: string[], save: boolean) => App.CopyIntoWorktree(id, files, save),
  /** Whether a project can use worktrees, its branches and default base. */
  projectGit: (projectId: string): Promise<ProjectGit> => App.ProjectGit(projectId),
  sessionDiff: (id: string): Promise<DiffResult> => App.SessionDiff(id),
  worktreeDiff: (id: string): Promise<DiffResult> => App.WorktreeDiff(id),
  listWorktrees: (projectId: string): Promise<WorktreeInfo[]> => App.ListWorktrees(projectId),
  /** dryRun: only report what would be lost (result.loss). */
  removeWorktree: (id: string, o: { force?: boolean; deleteBranch?: boolean; dryRun?: boolean }) =>
    App.RemoveWorktree(id, !!o.force, !!o.deleteBranch, !!o.dryRun),
  /** Ended agent sessions matching every word of query ('' = the latest); projectId '' = all. */
  searchHistory: (query: string, projectId = '', limit = 50): Promise<HistoryHit[]> => App.SearchHistory(query, projectId, limit),
  historyDetail: (id: string): Promise<HistoryDetail> => App.HistoryDetail(id),
  deleteHistory: (id: string): Promise<void> => App.DeleteHistory(id),
  /** A new session reopening an archived session's agent conversation. */
  resumeHistory: (id: string, cols: number, rows: number): Promise<Session> => App.ResumeHistory(id, cols, rows) as Promise<Session>,
  /** Removes the project's orphaned worktrees already merged and clean, with their branches. */
  removeMergedWorktrees: (projectId: string, dryRun = false): Promise<RemoveMergedResult> =>
    App.RemoveMergedWorktrees(projectId, dryRun),
  mergeWorktree: (id: string): Promise<MergeResult> => App.MergeWorktree(id),
  /** Pushes the branch and returns the pull request page; dryRun: only remote and URL. */
  pullRequest: (id: string, dryRun = false): Promise<PRResult> => App.PullRequestWorktree(id, dryRun),
  /** Code host pull requests, for a session's (sessionId) or a worktree's branch. */
  /** branch: another branch the session worked on ('' = the one it is on). */
  forgeStatus: (sessionId: string, worktreeId: string, branch = ''): Promise<ForgeStatus> =>
    App.ForgeStatus(sessionId, worktreeId, branch),
  forgeComments: (sessionId: string, worktreeId: string, n: number, branch = ''): Promise<ForgeComment[]> =>
    App.ForgeComments(sessionId, worktreeId, branch, n),
  /** The pull requests of every branch the session's folder worked on. */
  forgeList: (sessionId: string, worktreeId: string, refresh = false): Promise<ForgeList> =>
    App.ForgeList(sessionId, worktreeId, refresh),
  /** head: the branch the PR was shown for (refused if the folder moved). */
  forgeMerge: (sessionId: string, worktreeId: string, n: number, method: string, head: string, deleteBranch: boolean, branch = ''): Promise<ForgeStatus> =>
    App.ForgeMerge(sessionId, worktreeId, branch, n, method, head, deleteBranch),
  forgeCreate: (sessionId: string, worktreeId: string, title: string, body: string, draft: boolean): Promise<ForgeStatus> =>
    App.ForgeCreate(sessionId, worktreeId, title, body, draft),
  /** Messages sent by or to a session (panal msg). */
  listMessages: (sessionId: string): Promise<Message[]> => App.ListMessages(sessionId, ''),
  /** From the user to a session (id). */
  sendMessage: (to: string, body: string): Promise<Message[]> => App.SendMessage(to, '', body),
  /** Types a fixed line telling the agent to read its messages; returns how many were pending. */
  deliverMessages: (sessionId: string): Promise<number> => App.DeliverMessages(sessionId),
  /** Every message of a project, oldest first. */
  listProjectMessages: (projectId: string): Promise<Message[]> => App.ListMessages('', projectId),
  /** From the user to every session of a project. */
  sendMessageAll: (projectId: string, body: string): Promise<Message[]> => App.SendMessage('all', projectId, body),
  /** Delivers every held message of a project (manual mode); returns for how many sessions. */
  releaseMessages: (projectId: string): Promise<number> => App.ReleaseMessages(projectId),
  /** Opens a worktree (id) or a session's folder in the editor or Finder. */
  openFolder: (target: 'editor' | 'finder', ref: { worktreeId?: string; sessionId?: string }) =>
    App.OpenWorktree(ref.worktreeId ?? '', ref.sessionId ?? '', target),
  kill: (id: string) => App.Kill(id),
  /** Respawns a suspended session (agents continue their conversation when they can). */
  resume: (id: string): Promise<Session> => App.Resume(id),
  /** Forgets a suspended session and its saved scrollback. */
  discard: (id: string): Promise<Session> => App.Discard(id),
  resumeAll: (): Promise<SessionBatch> => App.ResumeAll(),
  discardAll: (): Promise<SessionBatch> => App.DiscardAll(),
  attach: (id: string, cols: number, rows: number) => App.Attach(id, cols, rows),
  detach: (id: string) => App.Detach(id),
  /** Lasting paths for dropped files (screenshot thumbnails live in a temp folder macOS empties on drop). */
  stageDrops: async (paths: string[]): Promise<string[]> => (await App.StageDrops(paths)) ?? paths,
  /** Raw bytes to the PTY (keystrokes are UTF-8 encoded by the caller). */
  inputBytes: (id: string, data: Uint8Array) => App.InputBase64(id, bytesToB64(data)),
  resize: (id: string, cols: number, rows: number) => App.Resize(id, cols, rows),
  /** Last lines of output, ANSI-stripped (grid cards). */
  tail: (id: string, lines: number): Promise<string[]> => App.Tail(id, lines),
  /** Marks a session's attention as seen. */
  ack: (id: string) => App.Ack(id),
  /** Which sessions the window shows (every split pane): no notifications for them. */
  setVisibleSessions: (ids: string[]) => App.SetVisibleSessions(ids),
  /** Sets a session's display name ('' resets it to the profile's). */
  rename: (id: string, name: string): Promise<Session> => App.Rename(id, name),
  /** Session a notification click asked for before the UI was up ('' if none). */
  takePendingFocus: (): Promise<string> => App.TakePendingFocus(),
  /** What ui.language "auto" resolves to on the backend ("en" | "es"). */
  systemLanguage: (): Promise<string> => App.SystemLanguage(),
  updaterState: (): Promise<UpdaterState> => App.UpdaterState(),
  /** User-initiated check: Sparkle's own window shows the outcome. */
  checkForUpdates: () => App.CheckForUpdates(),
  /** Shows Sparkle's window for the update ("Install and Relaunch" once downloaded; the daemon keeps
   * running). false: still downloading in the background, nothing shown. */
  installUpdateNow: (): Promise<boolean> => App.InstallUpdateNow(),
  setAutomaticChecks: (on: boolean): Promise<UpdaterState> => App.SetAutomaticChecks(on),
  setAutomaticDownloads: (on: boolean): Promise<UpdaterState> => App.SetAutomaticDownloads(on),
  /** Whether the daemon runs another version than the app (after an update). */
  daemonStatus: (): Promise<DaemonStatus> => App.DaemonStatus(),
  onboarding: (): Promise<OnboardingInfo> => App.Onboarding(),
  requestNotifications: (): Promise<string> => App.RequestNotifications(),
  openNotificationSettings: (): Promise<void> => App.OpenNotificationSettings(),
  /** macOS's permission, re-read (state as onboarding's notifications; detail: why not, in English). */
  notificationStatus: (): Promise<{ state: string; detail?: string }> => App.NotificationStatus(),
  /** Shows a notification now; rejects with why it can't. */
  testNotification: (title: string, body: string): Promise<void> => App.TestNotification(title, body),
  /** Replaces the daemon with this version's; suspended sessions are resumed. */
  restartDaemon: (): Promise<SessionBatch> => App.RestartDaemon(),
}

/** "±3" for a session's changed files ('' when unknown). */
export function changesLabel(g: GitInfo | undefined | null): string {
  return g ? `±${g.files}` : ''
}

/** Last spawn mode chosen for a project (localStorage), else the default. */
// Per project, the mode last picked for an agent. The key changed in 0.6.1,
// when worktree became the default: choices made before (mostly the then
// default "same") no longer win over it.
const modeKey = (projectId: string) => `panal.spawnMode2.${projectId}`

export function rememberedMode(projectId: string, fallback: SpawnMode): SpawnMode {
  try {
    const v = localStorage.getItem(modeKey(projectId))
    if (v === 'same' || v === 'worktree') return v
  } catch {
    /* storage unavailable */
  }
  return fallback
}

export function rememberMode(projectId: string, mode: SpawnMode) {
  try {
    localStorage.setItem(modeKey(projectId), mode)
  } catch {
    /* storage unavailable */
  }
}

/** Profiles that belong in a spawn menu (enabled, installed, valid). */
export function launchable(ps: Profile[]): Profile[] {
  // A daemon older than the catalog sends neither flag: offer everything.
  return ps.filter((p) => p.enabled !== false && p.installed !== false && !p.error)
}

/** Splits a command line into arguments (quotes and backslashes like sh). */
export function splitArgs(s: string): string[] {
  const out: string[] = []
  let cur = ''
  let have = false
  let quote: '"' | "'" | null = null
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if (quote) {
      if (c === quote) quote = null
      else if (c === '\\' && quote === '"' && i + 1 < s.length) cur += s[++i]
      else cur += c
    } else if (c === '"' || c === "'") {
      quote = c
      have = true
    } else if (c === '\\' && i + 1 < s.length) {
      cur += s[++i]
      have = true
    } else if (/\s/.test(c)) {
      if (have) out.push(cur)
      cur = ''
      have = false
    } else {
      cur += c
      have = true
    }
  }
  if (have) out.push(cur)
  return out
}

/** Inverse of splitArgs. */
export function joinArgs(args: string[] | undefined): string {
  return (args ?? []).map((a) => (a === '' || /[\s'"\\]/.test(a) ? `'${a.replace(/'/g, `'\\''`)}'` : a)).join(' ')
}

/** joinArgs for tooltips: the instructions panal hands codex
 * (developer_instructions=…) are shortened, they can be long. */
export function displayArgs(args: string[] | undefined): string {
  return joinArgs((args ?? []).map((a) => (a.startsWith('developer_instructions=') ? 'developer_instructions=…' : a)))
}

/** A daemon refusal with a stable code (see internal/apierr). The app
 * passes it as JSON in the error message (app/errors.go). */
export interface CodedError {
  message: string
  code: string
  data?: Record<string, unknown>
}

export function parseCodedError(msg: string): CodedError | null {
  if (!msg.startsWith('{')) return null
  try {
    const v = JSON.parse(msg) as Partial<CodedError>
    if (typeof v.code === 'string' && typeof v.message === 'string') return v as CodedError
  } catch {
    /* not JSON */
  }
  return null
}

function preview(list: unknown, n: number): string {
  const xs = Array.isArray(list) ? list.map(String) : []
  return xs.length <= n ? xs.join(', ') : `${xs.slice(0, n).join(', ')}, … +${xs.length - n}`
}

/** Localized text for a coded error, or null for codes we do not know. */
export function codedMessage(ce: CodedError): string | null {
  const d = ce.data ?? {}
  const p: Params = {}
  for (const [k, v] of Object.entries(d)) if (typeof v === 'string' || typeof v === 'number') p[k] = v
  if (typeof d.count === 'number') p.n = d.count
  let key = `err.${ce.code}`
  switch (ce.code) {
    case 'live_sessions': {
      const list = Array.isArray(d.sessions) ? (d.sessions as Array<{ id?: string; profileId?: string }>) : []
      p.sessions = list.map((x) => `${x.id} (${x.profileId})`).join(', ')
      break
    }
    case 'worktree_dirty':
      p.n = Array.isArray(d.uncommitted) ? d.uncommitted.length : 0
      p.files = preview(d.uncommitted, 5)
      break
    case 'worktree_unmerged':
      p.n = Array.isArray(d.unmerged) ? d.unmerged.length : 0
      break
    case 'merge_wrong_branch':
      if (!d.current) key = 'err.merge_wrong_branch_detached'
      break
    case 'pr_no_remote':
      if (Array.isArray(d.remotes) && d.remotes.length) {
        key = 'err.pr_no_remote_many'
        p.remotes = d.remotes.map(String).join(', ')
      }
      break
    case 'merge_conflict':
      p.files = preview(d.files, 5)
      break
    case 'profile_not_installed':
      if (d.install) key = 'err.profile_not_installed_hint'
      break
    case 'base_missing':
      if (!d.base) key = 'err.base_missing_none'
      break
    case 'resume_failed':
      key = `err.resume_failed.${String(d.reason ?? '')}`
      break
  }
  return hasKey(key) ? t(key, p) : null
}

/** User-facing text for a failed call: coded daemon refusals in the UI
 * language, anything else as reported (English from the daemon). */
export function errorMessage(e: unknown): string {
  const msg = e instanceof Error ? e.message : String(e)
  const ce = parseCodedError(msg)
  if (ce) return codedMessage(ce) ?? ce.message
  if (/unknown method "(profile|settings)\./.test(msg)) return t('err.oldDaemon')
  return msg
}

/** Decodes a base64 PTY chunk into raw bytes (xterm decodes UTF-8 itself,
 * statefully, so multi-byte sequences split across chunks stay intact). */
export function b64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

export function bytesToB64(bytes: Uint8Array): string {
  let bin = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(bin)
}
