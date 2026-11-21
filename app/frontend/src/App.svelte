<script lang="ts">
  import { keyLabel } from './lib/platform'
  import { permHelp, permLabel } from './lib/permission'
  import { onDestroy, onMount, tick } from 'svelte'
  import { EventsOn, WindowSetTitle } from '../wailsjs/runtime/runtime'
  import Changes from './lib/Changes.svelte'
  import Messages from './lib/Messages.svelte'
  import { showMessagesTab } from './lib/messages'
  import { ackOnView } from './lib/attention'
  import Grid from './lib/Grid.svelte'
  import ProgressBar from './lib/ProgressBar.svelte'
  import Settings from './lib/Settings.svelte'
  import History from './lib/History.svelte'
  import Conversation from './lib/Conversation.svelte'
  import { pruneActivity, recordSent, type MsgActivity } from './lib/msgactivity'
  import Sidebar from './lib/Sidebar.svelte'
  import ProjectBadge from './lib/ProjectBadge.svelte'
  import type { ProjectPatch } from './lib/badge'
  import Welcome from './lib/Welcome.svelte'
  import { activityText, mergeLive, progressOf, timerInfo } from './lib/activity'
  import { shortBranch } from './lib/branch'
  import TrustSetup from './lib/TrustSetup.svelte'
  import Handoff from './lib/Handoff.svelte'
  import { topLevel } from './lib/handoff'
  import { setLangSetting, setSystemLang, t, th } from './lib/i18n.svelte'
  import { TerminalManager } from './lib/terminals'
  import {
    MAX_PANES,
    assign,
    close as closeLeaf,
    cycle,
    focusedLeaf,
    geometry,
    leaves,
    loadLayout,
    neighbor,
    prune,
    saveLayout,
    sessionsOf,
    setRatio,
    single,
    split,
    type Dir,
    type Divider,
    type Layout,
    type Rect,
  } from './lib/panes'
  import { setAppearance, setTermPrefs, termPrefs } from './lib/prefs.svelte'
  import { arrangeWithin, moveBy } from './lib/reorder'
  import { shortcutOf, type Toward } from './lib/shortcuts'
  import { DEFAULT_FONT_SIZE, MAX_FONT_SIZE, MIN_FONT_SIZE, themeOf } from './lib/themes'
  import { shouldAutoRestart } from './lib/daemon'
  import {
    availableUpdate,
    dismissUpdate,
    setAvailableUpdate,
    updateBannerVisible,
    updateDownloading,
    updateNow,
  } from './lib/updates.svelte'
  import {
    api,
    attentionLabel,
    errorMessage,
    displayArgs,
    launchable,
    needsAttention,
    parseCodedError,
    rememberedMode,
    sessionTitle,
    type DaemonEvent,
    type Profile,
    type Project,
    type Session,
    type SpawnMode,
    type SpawnOptions,
    type WorktreeInfo,
    type DaemonStatus,
  } from './lib/api'
  import { SHELL_PROFILE, folderSharers, inProjectFolder } from './lib/shared'
  import { proposedMode } from './lib/spawnmode'
  import { prettyModel } from './lib/models'

  let projects = $state<Project[]>([])
  let sessions = $state<Session[]>([])
  let profiles = $state<Profile[]>([])
  /** The focused pane's session. */
  let selectedId = $state<string | null>(null)
  let currentProjectId = $state<string | null>(null)
  let error = $state<string | null>(null)
  let daemonDown = $state<string | null>(null)
  /** The daemon runs another version (left over from before an update). */
  let daemonOld = $state<DaemonStatus | null>(null)
  let restartingDaemon = $state(false)
  let loaded = $state(false)
  let settingsOpen = $state(false)
  type SettingsTab = 'general' | 'agents'
  let settingsTab = $state<SettingsTab>('general')
  /** The past-sessions view; historyStamp bumps on history.changed. */
  let historyOpen = $state(false)
  let historyStamp = $state(0)
  /** Who wrote to whom lately (sidebar), the open project conversation and
   * its reload stamp, and messages.auto_deliver (projects that did not choose). */
  let msgActivity = $state<Record<string, MsgActivity>>({})
  let convoProject = $state<string | null>(null)
  let convoStamp = $state(0)
  let defaultAutoMessages = $state(true)
  const msgWaiting = $derived.by(() => {
    const out: Record<string, number> = {}
    for (const s of sessions) if (s.msgPending) out[s.projectId] = (out[s.projectId] ?? 0) + s.msgPending
    return out
  })
  let gridOpen = $state(false)
  /** Project the grid shows (null: the current one). */
  let gridProjectId = $state<string | null>(null)
  /** Ticks every second for the elapsed timers. */
  let now = $state(Date.now())
  /** panal worktrees per project (for the orphan lists). */
  let worktrees = $state<Record<string, WorktreeInfo[]>>({})
  /** Which tab each session shows: terminal (default) or its changes. */
  let views = $state<Record<string, 'terminal' | 'changes' | 'messages'>>({})
  /** Bumped per session on message.changed: the Messages tab reloads. */
  let msgStamps = $state<Record<string, number>>({})
  /** An orphaned worktree's changes shown over the main area. */
  let orphanView = $state<{ id: string; remove: boolean } | null>(null)
  /** settings git.default_mode (what ⌘N / the + menu start with). */
  let defaultMode: SpawnMode = 'worktree'
  /** Split-pane layout per project (kept in localStorage). */
  let layouts = $state<Record<string, Layout>>({})
  /** The divider being dragged (its split id). */
  let dragging = $state<string | null>(null)
  /** ⌘F: the focused pane's search bar. */
  let search = $state({ open: false, session: '', query: '', caseSensitive: false, index: -1, count: 0 })
  let searchInput = $state<HTMLInputElement | null>(null)
  let searchOff: (() => void) | null = null

  let panesEl: HTMLDivElement
  let parkingEl: HTMLDivElement
  let errorTimer: number | undefined
  let fontSaveTimer: number | undefined

  const selected = $derived(sessions.find((s) => s.id === selectedId) ?? null)
  const currentProject = $derived(projects.find((p) => p.id === currentProjectId) ?? null)
  const attentionCount = $derived(sessions.filter(needsAttention).length)
  /** What the spawn menus offer, in order (the first one is ⌘N's). */
  const spawnable = $derived(launchable(profiles))
  const selectedTimer = $derived(selected ? timerInfo(selected, now, true) : null)
  /** The selected session has a git folder: offer the Cambios tab. */
  const hasChanges = $derived(!!selected && (selected.mode === 'worktree' || !!selected.git))
  /** The project has other sessions (or this one has messages): offer Mensajes. */
  const hasMessages = $derived(!!selected && showMessagesTab(selected, sessions, msgStamps[selected.id] ?? 0))
  const view = $derived.by(() => {
    const v = selected ? (views[selected.id] ?? 'terminal') : 'terminal'
    if (v === 'changes' && !hasChanges) return 'terminal'
    if (v === 'messages' && !hasMessages) return 'terminal'
    return v
  })

  const layout = $derived(currentProjectId ? (layouts[currentProjectId] ?? null) : null)
  const geo = $derived(layout ? geometry(layout.root) : null)
  const paneList = $derived(layout && geo ? leaves(layout.root).map((leaf) => ({ leaf, rect: geo.rects.get(leaf.id)! })) : [])
  /** Innermost first, so an outer divider is on top where two meet. */
  const dividers = $derived(geo ? [...geo.dividers].reverse() : [])
  const multi = $derived(paneList.length > 1)
  /** Sessions shown in a pane: the user is looking at them. */
  const visibleIds = $derived(layout ? sessionsOf(layout) : [])
  const termBg = $derived(themeOf(termPrefs().theme).colors.background)

  $effect(() => {
    void api.setVisibleSessions(visibleIds).catch(() => {})
  })

  $effect(() => {
    terms.setPrefs(termPrefs())
  })

  function setView(v: 'terminal' | 'changes' | 'messages') {
    if (!selected) return
    views[selected.id] = v
    if (v === 'terminal') terms.focus(selected.id)
    else if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  }

  async function refreshWorktrees(projectId: string) {
    try {
      const list = await api.listWorktrees(projectId)
      worktrees = { ...worktrees, [projectId]: list }
    } catch {
      /* older daemon or not a repo: no list */
    }
  }

  function refreshAllWorktrees() {
    for (const p of projects) void refreshWorktrees(p.id)
  }

  function openOrphan(id: string, remove = false) {
    gridOpen = false
    orphanView = { id, remove }
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  }

  function closeOrphan() {
    orphanView = null
    terms.focus()
  }

  $effect(() => {
    WindowSetTitle(attentionCount ? `panal (${attentionCount})` : 'panal')
  })

  /** Acks a session if the user can actually see it (in a pane); a pending
   * approval or question stays flagged (ackOnView). */
  function ackIfVisible(id: string | null) {
    if (!id || !document.hasFocus() || !visibleIds.includes(id)) return
    const s = sessions.find((x) => x.id === id)
    if (s && ackOnView(s)) void api.ack(id).catch(() => {})
  }

  function isAppShortcut(e: KeyboardEvent): boolean {
    return shortcutOf(e) !== null
  }

  const terms = new TerminalManager(isAppShortcut, showError, onTermFocus)

  function showError(msg: string) {
    error = msg
    clearTimeout(errorTimer)
    errorTimer = window.setTimeout(() => (error = null), 6000)
  }

  // A new worktree whose .panal.yaml has a setup command not trusted
  // yet: ask (TrustSetup), then spawn again with the answer.
  let setupAsk = $state<{ command: string; file: string; resolve: (c: 'trust' | 'skip' | null) => void } | null>(null)
  function askSetup(command: string, file: string): Promise<'trust' | 'skip' | null> {
    return new Promise((resolve) => {
      setupAsk = {
        command,
        file,
        resolve: (c) => {
          setupAsk = null
          resolve(c)
        },
      }
    })
  }

  // A new worktree of a repository without .panal.yaml lacks the .env
  // files git ignores: offer to copy them (once, or into a new file).
  let copyOffer = $state<{ sessionId: string; worktreeId: string; files: string[]; busy?: boolean; done?: string } | null>(null)
  async function acceptCopy(save: boolean) {
    const o = copyOffer
    if (!o) return
    copyOffer = { ...o, busy: true }
    const r = await guard(api.copyIntoWorktree(o.worktreeId, o.files, save))
    if (!r) {
      copyOffer = { ...o, busy: false }
      return
    }
    const done = r.created ? t('setup.copiedSaved', { n: r.copied.length, file: r.created }) : t('setup.copied', { n: r.copied.length })
    copyOffer = { ...o, busy: false, done }
    window.setTimeout(() => copyOffer?.sessionId === o.sessionId && (copyOffer = null), 6000)
  }

  /** api.spawn, asking first when the worktree's setup is not trusted yet. */
  async function spawnSession(projectId: string, profileId: string, cols: number, rows: number, opts?: Partial<SpawnOptions>) {
    let info: Session | undefined
    try {
      info = await api.spawn(projectId, profileId, cols, rows, opts)
    } catch (e) {
      const ce = parseCodedError(e instanceof Error ? e.message : String(e))
      if (ce?.code !== 'setup_untrusted') {
        showError(errorMessage(e))
        return undefined
      }
      const d = (ce.data ?? {}) as Record<string, string>
      const choice = await askSetup(d.command ?? '', d.file ?? '.panal.yaml')
      if (!choice) return undefined
      info = await guard(api.spawn(projectId, profileId, cols, rows, { ...opts, trustSetup: choice === 'trust', skipSetup: choice === 'skip' }))
    }
    if (info?.suggestCopy?.length && info.worktreeId) {
      copyOffer = { sessionId: info.id, worktreeId: info.worktreeId, files: info.suggestCopy }
    }
    return info
  }

  async function guard<T>(p: Promise<T>): Promise<T | undefined> {
    try {
      return await p
    } catch (e) {
      showError(errorMessage(e))
      return undefined
    }
  }

  // --- Panes ------------------------------------------------------------------

  function setLayout(projectId: string, l: Layout) {
    layouts[projectId] = l
    saveLayout(projectId, l)
  }

  /** The project's layout: the stored one (without sessions that are gone)
   * or a single empty pane. Call once sessions are loaded. */
  function ensureLayout(projectId: string): Layout {
    let l = layouts[projectId]
    if (!l) {
      const stored = loadLayout(projectId)
      l = stored ? prune(stored, new Set(sessions.filter((s) => s.projectId === projectId).map((s) => s.id))) : single(null)
      layouts[projectId] = l
    }
    return l
  }

  /** Drops panes of sessions that no longer exist, in every project. */
  function pruneLayouts() {
    const alive = new Set(sessions.map((s) => s.id))
    for (const [pid, l] of Object.entries(layouts)) {
      const next = prune(l, alive)
      if (next !== l) setLayout(pid, next)
    }
  }

  /** After sessions went away: keep the selection on the focused pane. */
  function reconcileSelection() {
    const l = currentProjectId ? layouts[currentProjectId] : undefined
    const f = l ? focusedLeaf(l).session : null
    if (f && sessions.some((s) => s.id === f)) {
      if (f !== selectedId) select(f, true)
      return
    }
    if (!selectedId || !sessions.some((s) => s.id === selectedId)) select(fallbackSelection(), true)
  }

  function rectStyle(r: Rect): string {
    return `left:${r.x * 100}%;top:${r.y * 100}%;width:${r.w * 100}%;height:${r.h * 100}%`
  }

  function dividerStyle(d: Divider): string {
    const r = d.rect
    return d.dir === 'row'
      ? `left:${(r.x + r.w * d.ratio) * 100}%;top:${r.y * 100}%;height:${r.h * 100}%`
      : `top:${(r.y + r.h * d.ratio) * 100}%;left:${r.x * 100}%;width:${r.w * 100}%`
  }

  /** Puts a session's terminal into a pane's slot (Svelte attachment). */
  function termSlot(id: string) {
    return (node: HTMLElement) => {
      terms.place(id, node)
      return () => terms.unplace(id, node)
    }
  }

  function focusLeaf(leafId: string) {
    const pid = currentProjectId
    const l = pid ? layouts[pid] : undefined
    if (!pid || !l) return
    const leaf = leaves(l.root).find((x) => x.id === leafId)
    if (!leaf) return
    if (l.focus !== leafId) setLayout(pid, { ...l, focus: leafId })
    if (leaf.session) {
      if (leaf.session !== selectedId) select(leaf.session, true)
      else terms.focus(leaf.session)
    } else {
      selectedId = null
    }
  }

  /** A terminal got the keyboard (clicked): its pane becomes the focused one. */
  function onTermFocus(id: string) {
    const pid = currentProjectId
    const l = pid ? layouts[pid] : undefined
    const leaf = l ? leaves(l.root).find((x) => x.session === id) : undefined
    if (!pid || !l || !leaf) return
    if (l.focus !== leaf.id) setLayout(pid, { ...l, focus: leaf.id })
    if (selectedId !== id) {
      if (search.open && search.session !== id) closeSearch(false)
      selectedId = id
      ackIfVisible(id)
    }
  }

  /** The size a terminal gets in a new pane next to the focused one. */
  function splitSize(dir: Dir): { cols: number; rows: number } {
    const cur = selectedId ? terms.size(selectedId) : null
    if (cur) return dir === 'row' ? { cols: Math.max(Math.floor(cur.cols / 2) - 1, 20), rows: cur.rows } : { cols: cur.cols, rows: Math.max(Math.floor(cur.rows / 2) - 1, 5) }
    return paneSize()
  }

  /** The size of a terminal filling the focused pane. */
  function paneSize(): { cols: number; rows: number } {
    const cur = selectedId ? terms.size(selectedId) : null
    if (cur) return cur
    const r = panesEl?.getBoundingClientRect()
    return terms.proposeSize(r?.width ?? 0, r?.height ?? 0)
  }

  /** ⌘D / ⌘⇧D: a new session of the focused one's profile and mode, in a
   * new pane right of / below it. */
  async function splitPane(dir: Dir) {
    const pid = currentProjectId
    if (!pid) return
    const l = ensureLayout(pid)
    if (leaves(l.root).length >= MAX_PANES) {
      showError(t('panes.max', { n: MAX_PANES }))
      return
    }
    const cur = selected && selected.projectId === pid ? selected : null
    const prof = (cur && spawnable.find((p) => p.id === cur.profileId)) || spawnable[0]
    if (!prof) {
      showError(t('panes.noProfile'))
      return
    }
    // Like the current session, but a second agent in a busy project folder
    // gets a worktree (defaultModeFor), as with the + menu.
    let mode: SpawnMode
    if (cur?.mode === 'worktree') mode = 'worktree'
    else if (cur && (prof.id === SHELL_PROFILE || !folderSharers(sessions, pid).length)) mode = 'same'
    else mode = await defaultModeFor(pid, prof.id)
    const { cols, rows } = splitSize(dir)
    const info = await spawnSession(pid, prof.id, cols, rows, { mode })
    if (!info) return
    if (!sessions.some((s) => s.id === info.id)) sessions = [...sessions, info]
    const now = layouts[pid] ?? l
    const f = focusedLeaf(now)
    setLayout(pid, f.session ? split(now, f.id, dir, info.id) : assign(now, f.id, info.id))
    select(info.id)
  }

  /** ⌘W: closes the focused pane; its session keeps running. */
  function closePane(leafId?: string) {
    const pid = currentProjectId
    const l = pid ? layouts[pid] : undefined
    if (!pid || !l || leaves(l.root).length <= 1) return
    const next = closeLeaf(l, leafId ?? l.focus)
    setLayout(pid, next)
    if (search.open && !sessionsOf(next).includes(search.session)) closeSearch(false)
    const f = focusedLeaf(next).session
    if (f) select(f, true)
    else selectedId = null
  }

  function movePane(toward: Toward | 1 | -1) {
    const l = currentProjectId ? layouts[currentProjectId] : undefined
    if (!l || leaves(l.root).length <= 1) return
    const target = typeof toward === 'number' ? cycle(l, l.focus, toward) : neighbor(l, l.focus, toward)
    if (target) focusLeaf(target)
  }

  function startDrag(ev: PointerEvent, d: Divider) {
    const pid = currentProjectId
    if (!pid || !panesEl || ev.button !== 0) return
    ev.preventDefault()
    const box = panesEl.getBoundingClientRect()
    const target = ev.currentTarget as HTMLElement
    target.setPointerCapture(ev.pointerId)
    dragging = d.id
    const r = d.rect
    const move = (e: PointerEvent) => {
      const ratio =
        d.dir === 'row' ? ((e.clientX - box.left) / box.width - r.x) / r.w : ((e.clientY - box.top) / box.height - r.y) / r.h
      const l = layouts[pid]
      if (l) layouts[pid] = setRatio(l, d.id, ratio)
    }
    const up = () => {
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', up)
      target.removeEventListener('pointercancel', up)
      dragging = null
      const l = layouts[pid]
      if (l) saveLayout(pid, l)
      terms.focus()
    }
    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', up)
    target.addEventListener('pointercancel', up)
  }

  // --- Search -----------------------------------------------------------------

  async function openSearch() {
    if (!selectedId || view !== 'terminal') return
    if (search.open && search.session === selectedId) {
      searchInput?.focus()
      searchInput?.select()
      return
    }
    closeSearch(false)
    const id = selectedId
    search.open = true
    search.session = id
    search.index = -1
    search.count = 0
    searchOff = terms.onSearchResults(id, (r) => {
      search.index = r.index
      search.count = r.count
    })
    await tick()
    searchInput?.focus()
    searchInput?.select()
    if (search.query) runSearch(false)
  }

  function runSearch(backwards: boolean, incremental = false) {
    if (!search.open) return
    if (!search.query) {
      terms.clearSearch(search.session)
      search.index = -1
      search.count = 0
      return
    }
    const found = terms.find(search.session, search.query, { caseSensitive: search.caseSensitive, backwards, incremental })
    if (!found) {
      search.index = -1
      search.count = 0
    }
  }

  function closeSearch(refocus = true) {
    if (!search.open) return
    searchOff?.()
    searchOff = null
    terms.clearSearch(search.session)
    search.open = false
    if (refocus) terms.focus(selectedId)
  }

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      closeSearch()
    } else if (e.key === 'Enter') {
      e.preventDefault()
      runSearch(e.shiftKey)
    }
  }

  const searchStatus = $derived(
    !search.query
      ? ''
      : search.count === 0
        ? t('search.none')
        : search.index >= 0
          ? t('search.count', { i: search.index + 1, n: search.count })
          : t('search.many', { n: search.count }),
  )

  // --- Font size --------------------------------------------------------------

  /** ⌘= / ⌘- / ⌘⌥0: terminal.font_size, applied now, saved shortly after. */
  function bumpFont(delta: 1 | -1 | 0) {
    const p = termPrefs()
    const size = delta === 0 ? DEFAULT_FONT_SIZE : Math.min(Math.max(p.fontSize + delta, MIN_FONT_SIZE), MAX_FONT_SIZE)
    if (size === p.fontSize) return
    setTermPrefs({ ...p, fontSize: size })
    clearTimeout(fontSaveTimer)
    fontSaveTimer = window.setTimeout(() => {
      void api.saveSettings({ terminal: termPrefs() }).catch((e) => showError(errorMessage(e)))
    }, 500)
  }

  // --- Sessions & projects ----------------------------------------------------

  async function refreshSessions() {
    const list = await guard(api.listSessions())
    if (!list) return
    sessions = list
    terms.retain(new Set(list.map((s) => s.id)))
    terms.revive(list)
    pruneLayouts()
    if (selectedId && !list.some((s) => s.id === selectedId)) reconcileSelection()
    // Nothing selected and a session appeared in the current project (e.g.
    // spawned from the CLI): show it rather than "No sessions" next to it.
    else if (loaded && !selectedId && list.some((s) => s.projectId === currentProjectId)) select(fallbackSelection(), true)
  }

  async function refreshAll() {
    const [ps, profs] = await Promise.all([guard(api.listProjects()), guard(api.listProfiles())])
    if (ps) projects = ps
    if (profs) profiles = profs
    await refreshSessions()
    if (!currentProjectId || !projects.some((p) => p.id === currentProjectId)) {
      currentProjectId = projects[0]?.id ?? null
    }
    if (!selectedId) {
      // Restore the project's panes (their focused session) if it has any.
      const f = currentProjectId ? focusedLeaf(ensureLayout(currentProjectId)).session : null
      select(f ?? fallbackSelection(), true)
    }
    loaded = true
    refreshAllWorktrees()
    void api
      .getSettings()
      .then((st) => {
        defaultMode = st.git?.defaultMode === 'same' ? 'same' : 'worktree'
        defaultAutoMessages = st.messages?.autoDeliver ?? true
        setLangSetting(st.ui?.language)
        setAppearance(st.ui?.appearance)
        setTermPrefs(st.terminal)
      })
      .catch(() => {})
    void api
      .daemonStatus()
      .then((s) => {
        daemonOld = s.mismatch || s.newer ? s : null
        // Right after an update: restart the old daemon unasked when no
        // agent would be cut off (the banner shows "Restarting…").
        if (shouldAutoRestart(s, busyCount, autoRestartTried)) {
          autoRestartTried = true
          void restartDaemon()
        }
      })
      .catch(() => {})
  }

  /** Agents in the middle of a turn, which a daemon restart interrupts. */
  const busyCount = $derived(sessions.filter((s) => s.state === 'running').length)
  let confirmRestart = $state(false)
  /** The app already restarted an old daemon on its own this run. */
  let autoRestartTried = false

  /** "Restart now": replace the old daemon; its sessions are resumed.
   * Asks first when agents are working. */
  async function restartDaemon(force = false) {
    if (!force && busyCount > 0) {
      confirmRestart = true
      return
    }
    confirmRestart = false
    restartingDaemon = true
    const b = await guard(api.restartDaemon())
    restartingDaemon = false
    if (b) {
      daemonOld = null
      if (b.failed?.length) showError(b.failed.map((f) => `${f.sessionId}: ${f.error}`).join('\n'))
    }
    void refreshAll()
  }

  /** A session of the current project not already shown in a pane. */
  function fallbackSelection(): string | null {
    const shown = new Set(visibleIds)
    const inProject = sessions.filter((s) => s.projectId === currentProjectId && !shown.has(s.id))
    return (inProject.find((s) => s.state !== 'exited') ?? inProject[0] ?? sessions.find((s) => !shown.has(s.id)))?.id ?? null
  }

  /** Shows a session: focuses its pane if it is in one, else puts it in
   * the focused pane. */
  function select(id: string | null, keepGrid = false) {
    if (!keepGrid) gridOpen = false
    const s = id ? sessions.find((x) => x.id === id) : undefined
    if (s) currentProjectId = s.projectId
    const pid = currentProjectId
    if (pid) {
      const l = ensureLayout(pid)
      const hit = id ? leaves(l.root).find((x) => x.session === id) : undefined
      if (hit) {
        if (l.focus !== hit.id) setLayout(pid, { ...l, focus: hit.id })
      } else if (id || !multiPanes(l)) {
        setLayout(pid, assign(l, l.focus, id))
      }
    }
    if (search.open && search.session !== id) closeSearch(false)
    selectedId = id
    terms.focus(id)
    ackIfVisible(id)
  }

  function multiPanes(l: Layout): boolean {
    return leaves(l.root).length > 1
  }

  function openGrid(projectId?: string) {
    gridProjectId = projectId ?? null
    if (projectId) currentProjectId = projectId
    gridOpen = true
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  }

  function closeGrid() {
    gridOpen = false
    terms.focus()
  }

  function openFromGrid(id: string) {
    gridOpen = false
    select(id)
  }

  function selectProject(id: string) {
    currentProjectId = id
    if (gridOpen) gridProjectId = id // the grid follows the project
    if (selected?.projectId === id) return
    const f = focusedLeaf(ensureLayout(id)).session
    const first = f ?? sessions.find((s) => s.projectId === id)?.id
    if (first) select(first, gridOpen)
    else selectedId = null
  }

  /** Opens Settings on tab (General by default). */
  function openSettings(tab: SettingsTab = 'general') {
    settingsTab = tab
    settingsOpen = true
    closeSearch(false)
    // Keep keystrokes out of the (now covered) terminal.
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
    // Detection may be stale (an agent installed meanwhile): refresh.
    void guard(api.listProfiles()).then((ps) => ps && (profiles = ps))
  }

  function closeSettings() {
    settingsOpen = false
    terms.focus()
  }

  function openHistory() {
    if (settingsOpen) settingsOpen = false
    historyOpen = true
    closeSearch(false)
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  }

  function closeHistory() {
    historyOpen = false
    terms.focus()
  }

  /** Reopens an archived session's conversation in a new session. */
  async function resumeHistory(id: string) {
    const { cols, rows } = paneSize()
    let info: Session
    try {
      info = await api.resumeHistory(id, cols, rows)
    } catch (e) {
      throw new Error(errorMessage(e))
    }
    if (!sessions.some((s) => s.id === info.id)) sessions = [...sessions, info]
    historyOpen = false
    currentProjectId = info.projectId
    select(info.id)
  }

  /** A sidebar action that goes somewhere (a session, a project, the
   * overview…) closes Settings first, as × does (unsaved edits included). */
  function leavingSettings<A extends unknown[], R>(fn: (...args: A) => R): (...args: A) => R {
    return (...args) => {
      if (settingsOpen) closeSettings()
      if (historyOpen) closeHistory()
      convoProject = null // what fn opens takes the conversation's place
      return fn(...args)
    }
  }

  async function spawn(projectId: string, profileId: string, opts?: Partial<SpawnOptions>) {
    const { cols, rows } = paneSize()
    const info = await spawnSession(projectId, profileId, cols, rows, opts)
    if (!info) return
    if (!sessions.some((s) => s.id === info.id)) sessions = [...sessions, info]
    if (opts?.worktreeId) orphanView = null
    select(info.id)
  }

  /** Installs a catalog agent in a terminal session of the current project
   * (its output in view); the agent list reloads when it is found. */
  async function installAgent(profileId: string) {
    const pid = currentProjectId ?? projects[0]?.id
    if (!pid) {
      showError(t('settings.installNoProject'))
      return
    }
    const { cols, rows } = paneSize()
    const info = await guard(api.installAgent(profileId, pid, cols, rows))
    if (!info) return
    if (!sessions.some((s) => s.id === info.id)) sessions = [...sessions, info]
    closeSettings()
    select(info.id)
  }

  /** "Continue with…": the session whose work is handed over, and the preset. */
  let handoffFor = $state<{ session: Session; preset?: { profileId?: string; where?: SpawnMode } } | null>(null)
  function openHandoff(id: string, preset?: { profileId?: string; where?: SpawnMode }) {
    const s = sessions.find((x) => x.id === id)
    if (s) handoffFor = { session: s, preset }
  }
  function handedOff(info: Session) {
    handoffFor = null
    if (!sessions.some((s) => s.id === info.id)) sessions = [...sessions, info]
    else sessions = sessions.map((s) => (s.id === info.id ? { ...s, continuedFrom: info.continuedFrom } : s))
    select(info.id)
  }

  async function kill(id: string) {
    const info = await guard(api.kill(id))
    if (!info) return
    // A second kill of an exited session removes it; the event updates the list.
    sessions = sessions.map((s) => (s.id === id ? { ...s, ...info } : s))
  }

  async function rename(id: string, name: string) {
    const info = await guard(api.rename(id, name))
    if (info) sessions = sessions.map((s) => (s.id === id ? { ...s, name: info.name } : s))
  }

  async function resume(id: string) {
    const info = await guard(api.resume(id))
    if (info) sessions = sessions.map((s) => (s.id === id ? info : s))
  }

  async function discard(id: string) {
    await guard(api.discard(id)) // session.removed updates the list
  }

  async function resumeAll() {
    const b = await guard(api.resumeAll())
    if (b?.failed?.length) showError(b.failed.map((f) => `${f.sessionId}: ${f.error}`).join('\n'))
    void refreshSessions()
  }

  async function discardAll() {
    await guard(api.discardAll())
    void refreshSessions()
  }

  async function addProject() {
    const p = await guard(api.addProject())
    if (!p) return
    if (!projects.some((x) => x.id === p.id)) projects = [...projects, p]
    // Show the project, not the session that was open: a new one has no
    // sessions, so the empty view offers to start one.
    gridOpen = false
    selectProject(p.id)
  }

  async function removeProject(id: string) {
    // Its live sessions would keep running with no row left to reach them.
    const live = sessions.filter((s) => s.projectId === id && s.state !== 'exited').length
    if (live) {
      showError(t('sidebar.removeProjectLive', { n: live }))
      return
    }
    try {
      await api.removeProject(id)
    } catch (e) {
      showError(errorMessage(e))
      return
    }
    projects = projects.filter((p) => p.id !== id)
    if (currentProjectId === id) currentProjectId = projects[0]?.id ?? null
  }

  /** Drag & drop in the sidebar: shown at once, then confirmed by the daemon
   * (which tells other windows and the CLI). */
  async function reorderProjects(ids: string[]) {
    const prev = projects
    const byId = new Map(projects.map((p) => [p.id, p]))
    projects = ids.flatMap((id) => byId.get(id) ?? [])
    try {
      projects = await api.reorderProjects(ids)
    } catch (e) {
      projects = prev
      showError(errorMessage(e))
    }
  }

  /** The customize popover: rejects with the message it shows inline. */
  async function updateProject(id: string, patch: ProjectPatch) {
    try {
      const p = await api.updateProject(id, patch)
      projects = projects.map((x) => (x.id === p.id ? p : x))
    } catch (e) {
      throw new Error(errorMessage(e))
    }
  }

  async function reorderSessions(projectId: string, ids: string[]) {
    sessions = arrangeWithin(sessions, (s) => s.projectId === projectId, ids)
    try {
      await api.reorderSessions(projectId, ids)
    } catch (e) {
      showError(errorMessage(e))
      void refreshSessions()
    }
  }

  /** ⌘⇧↑ / ⌘⇧↓: moves the selected session within its project. */
  function moveSelected(delta: -1 | 1) {
    const sel = sessions.find((s) => s.id === selectedId)
    if (!sel) return
    const ids = sessions.filter((s) => s.projectId === sel.projectId).map((s) => s.id)
    const next = moveBy(ids, sel.id, delta)
    if (next) void reorderSessions(sel.projectId, next)
  }

  function onDaemonEvent(ev: DaemonEvent) {
    switch (ev.type) {
      case 'session.state':
      case 'session.attention':
        // Attention fields are omitted when cleared, so copy them as-is.
        sessions = sessions.map((s) =>
          s.id === ev.sessionId
            ? mergeLive(
                {
                  ...s,
                  state: ev.state ?? s.state,
                  exitCode: ev.exitCode ?? s.exitCode,
                  stopped: ev.stopped ?? s.stopped,
                  attentionKind: ev.attentionKind,
                  attentionMessage: ev.attentionMessage,
                  attentionAt: ev.attentionAt,
                  unread: ev.unread,
                },
                ev,
              )
            : s,
        )
        if (ev.raised) ackIfVisible(ev.sessionId)
        break
      case 'session.activity':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? mergeLive(s, ev) : s))
        break
      case 'session.git':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? { ...s, git: ev.git as Session['git'] } : s))
        break
      case 'session.model':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? { ...s, model: ev.model } : s))
        break
      case 'session.browser':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? { ...s, browserUrl: ev.browserUrl } : s))
        break
      case 'profile.changed':
        void guard(api.listProfiles()).then((ps) => ps && (profiles = ps))
        break
      case 'message.changed':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? { ...s, msgPending: ev.msgPending ?? 0, msgHold: ev.msgHold ?? '' } : s))
        msgStamps[ev.sessionId] = (msgStamps[ev.sessionId] ?? 0) + 1
        if (ev.projectId && ev.projectId === convoProject) convoStamp++
        break
      case 'message.sent': {
        const t0 = Date.now()
        msgActivity = recordSent(pruneActivity(msgActivity, t0), ev, t0)
        if (ev.projectId && ev.projectId === convoProject) convoStamp++
        break
      }
      case 'session.renamed':
        sessions = sessions.map((s) => (s.id === ev.sessionId ? { ...s, name: ev.sessionName } : s))
        break
      case 'worktree.changed':
        if (ev.projectId) void refreshWorktrees(ev.projectId)
        break
      case 'history.changed':
        historyStamp++
        break
      case 'project.changed':
        void guard(api.listProjects()).then((ps) => ps && (projects = ps))
        break
      case 'session.reordered':
        void refreshSessions()
        break
      case 'session.spawned':
        void refreshSessions()
        break
      case 'session.removed':
        sessions = sessions.filter((s) => s.id !== ev.sessionId)
        terms.dispose(ev.sessionId)
        if (search.session === ev.sessionId) closeSearch(false)
        pruneLayouts()
        if (selectedId === ev.sessionId) reconcileSelection()
        break
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && orphanView && !settingsOpen) {
      e.preventDefault()
      closeOrphan()
      return
    }
    const sc = shortcutOf(e)
    if (!sc) return
    e.preventDefault()
    if (sc.kind === 'settings') {
      if (historyOpen) historyOpen = false
      openSettings()
      return
    }
    if (sc.kind === 'history') {
      if (historyOpen) closeHistory()
      else openHistory()
      return
    }
    if (sc.kind === 'font') {
      bumpFont(sc.delta)
      return
    }
    if (settingsOpen || historyOpen || convoProject) return
    const covered = gridOpen || !!orphanView
    switch (sc.kind) {
      case 'grid':
        if (gridOpen) closeGrid()
        else openGrid()
        return
      case 'session': {
        const list = topLevel(sessions.filter((s) => s.projectId === currentProjectId))
        if (list[sc.n - 1]) select(list[sc.n - 1].id)
        return
      }
      case 'moveSession':
        moveSelected(sc.delta)
        return
      case 'new': {
        // ⌘N: new session in the current project with the first profile.
        const pid = currentProjectId ?? projects[0]?.id
        const prof = spawnable[0]
        if (pid && prof) void spawnDefault(pid, prof.id)
        return
      }
    }
    if (covered) return
    switch (sc.kind) {
      case 'split':
        void splitPane(sc.dir)
        return
      case 'closePane':
        closePane()
        return
      case 'cyclePane':
        movePane(sc.delta)
        return
      case 'movePane':
        movePane(sc.toward)
        return
      case 'find':
        void openSearch()
        return
      case 'findNext':
        if (search.open) runSearch(sc.backwards)
        else if (search.query) void openSearch().then(() => runSearch(sc.backwards))
        return
    }
  }

  /** The remembered (or default) mode, but like the + menu: a worktree when
   * another agent already works in the project folder (unless it's a plain
   * shell being started), and the same folder when the project cannot use
   * worktrees (not a git repo…). */
  async function defaultModeFor(projectId: string, profileId: string): Promise<SpawnMode> {
    const shell = profileId === SHELL_PROFILE
    const busy = !shell && folderSharers(sessions, projectId).length > 0
    const mode = proposedMode({ shell, busy, remembered: rememberedMode(projectId, defaultMode), fallback: defaultMode })
    if (mode === 'worktree') {
      const g = await api.projectGit(projectId).catch(() => null)
      if (!g?.isRepo || g.error) return 'same'
    }
    return mode
  }

  async function spawnDefault(projectId: string, profileId: string) {
    await spawn(projectId, profileId, { mode: await defaultModeFor(projectId, profileId) })
  }

  function focusFromNotification(id: string) {
    if (sessions.some((s) => s.id === id)) select(id)
    else void refreshSessions().then(() => sessions.some((s) => s.id === id) && select(id))
  }

  const offs: Array<() => void> = []
  onMount(() => {
    terms.mount(parkingEl)
    // Linux can't be asked whether the window is focused (macOS can): tell
    // the backend, which skips notifying about what is in view.
    const onWinFocus = () => void api.windowFocus(true)
    const onWinBlur = () => void api.windowFocus(false)
    window.addEventListener('focus', onWinFocus)
    window.addEventListener('blur', onWinBlur)
    if (document.hasFocus()) onWinFocus()
    offs.push(
      () => window.removeEventListener('focus', onWinFocus),
      () => window.removeEventListener('blur', onWinBlur),
      EventsOn('daemon:event', onDaemonEvent),
      EventsOn('daemon:resync', () => {
        daemonDown = null
        void refreshAll()
      }),
      EventsOn('daemon:down', (msg: string) => (daemonDown = msg)),
      // App menu "Preferences…" (⌘,).
      EventsOn('app:open-settings', () => openSettings()),
      // A notification was clicked.
      EventsOn('app:focus-session', focusFromNotification),
      // A newer release was found (null: none any more).
      EventsOn('updates.available', setAvailableUpdate),
    )
    // One found before the window loaded (the event may also win the race).
    void api
      .updaterState()
      .then((s) => s.available && setAvailableUpdate(s.available))
      .catch(() => {})
    // What ui.language "auto" means on this Mac.
    void api
      .systemLanguage()
      .then(setSystemLang)
      .catch(() => {})
    // A click on a daemon notification may have launched the app.
    void refreshAll().then(() =>
      guard(api.takePendingFocus()).then((id) => id && focusFromNotification(id)),
    )
    const onFocus = () => {
      void guard(api.listProjects()).then((ps) => ps && (projects = ps))
      for (const id of visibleIds) ackIfVisible(id)
    }
    window.addEventListener('focus', onFocus)
    offs.push(() => window.removeEventListener('focus', onFocus))
    const tick = window.setInterval(() => (now = Date.now()), 1000)
    offs.push(() => clearInterval(tick))
  })
  onDestroy(() => {
    offs.forEach((off) => off())
    terms.unmount()
  })
</script>

<svelte:window onkeydown={onKeydown} />

<div class="layout">
  <Sidebar
    {projects}
    {sessions}
    profiles={spawnable}
    {selectedId}
    {visibleIds}
    {currentProjectId}
    onSelect={leavingSettings(select)}
    onSelectProject={leavingSettings(selectProject)}
    onSpawn={leavingSettings(spawn)}
    onKill={kill}
    onHandoff={(id) => openHandoff(id)}
    onRename={rename}
    onResume={resume}
    onDiscard={discard}
    onResumeAll={resumeAll}
    onDiscardAll={discardAll}
    onAddProject={addProject}
    onRemoveProject={removeProject}
    onReorderProjects={reorderProjects}
    onUpdateProject={updateProject}
    onReorderSessions={reorderSessions}
    onOpenSettings={() => openSettings()}
    onOpenHistory={openHistory}
    {msgActivity}
    {defaultAutoMessages}
    {msgWaiting}
    onOpenConversation={leavingSettings((id: string) => {
      historyOpen = false
      convoProject = id
      convoStamp++
    })}
    onOpenGrid={leavingSettings(openGrid)}
    overviewOpen={gridOpen && !gridProjectId}
    {now}
    {worktrees}
    onOpenWorktree={leavingSettings(openOrphan)}
  />

  <main class="main">
    <header class="titlebar drag">
      {#if selected}
        {@const proj = projects.find((p) => p.id === selected.projectId)}
        {#if proj}<span class="title-badge" title={proj.name}><ProjectBadge project={proj} size={18} /></span>{/if}
        <span class="title" title={selected.cwd}>{sessionTitle(selected)}</span>
        {#if selected.name}<span class="profile">{selected.profileId}</span>{/if}
        {#if selected.model}<span class="profile" title={selected.model}>{prettyModel(selected.model)}</span>{/if}
        {#if selected.mode === 'worktree' && selected.branch}
          {@const cur = selected.git?.head || selected.branch}
          <span class="branch" title={`${cur}${cur !== selected.branch ? ` (${selected.branch})` : ''}\n${selected.worktreePath ?? ''}`}>⎇ {shortBranch(cur)}</span>
        {:else if selected.git?.head}
          <span class="branch same" title={t('sidebar.folderBranch', { branch: selected.git.head })}>⎇ {shortBranch(selected.git.head)}</span>
        {/if}
        {#if selected.port}
          <span class="port" title={t('sidebar.port', { port: selected.port })}>:{selected.port}</span>
        {/if}
        {#if permLabel(selected.permission)}
          <span class="perm {selected.permission}" title={permHelp(selected.permission)}>{permLabel(selected.permission)}</span>
        {/if}
        {#if selected.attentionKind && selected.state !== 'exited'}
          <span class="attn {selected.attentionKind}" title={selected.attentionMessage ?? ''}>
            {attentionLabel(selected.attentionKind)}{#if selected.attentionMessage}: {selected.attentionMessage}{/if}
          </span>
        {/if}
        {#if selected.state !== 'exited' && selected.activity}
          {@const act = activityText(selected)}
          <span class="activity" title={act}>{act}</span>
        {/if}
        <span class="grow"></span>
        {#if selectedTimer?.text}
          <span class="timer" class:stalled={selectedTimer.stalled}>{selectedTimer.text}</span>
        {/if}
      {:else if currentProject}
        <span class="title-badge"><ProjectBadge project={currentProject} size={18} /></span>
        <span class="title">{currentProject.name}</span>
        <span class="subtitle">{currentProject.path}</span>
      {/if}
    </header>

    {#if selected && (hasChanges || hasMessages)}
      <nav class="view-tabs">
        <button class:on={view === 'terminal'} onclick={() => setView('terminal')}>{t('view.terminal')}</button>
        {#if hasChanges}
          <button class:on={view === 'changes'} onclick={() => setView('changes')}
            >{t('view.changes')}{selected.git ? ` (${selected.git.files})` : ''}</button
          >
        {/if}
        {#if hasMessages}
          <button class:on={view === 'messages'} onclick={() => setView('messages')}
            >{t('view.messages')}{selected.msgPending ? ` (${selected.msgPending})` : ''}</button
          >
        {/if}
      </nav>
    {/if}

    <div class="term-area">
      <div class="panes" class:multi class:dragging={!!dragging} bind:this={panesEl}>
        {#each paneList as p (p.leaf.id)}
          {@const s = p.leaf.session ? sessions.find((x) => x.id === p.leaf.session) : undefined}
          {@const focused = p.leaf.id === layout?.focus}
          {@const prog = s && s.state !== 'exited' ? progressOf(s) : undefined}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="pane" class:focused style={rectStyle(p.rect)} onmousedown={() => !focused && focusLeaf(p.leaf.id)}>
            <div class="pane-frame">
              {#if multi}
                <div class="pane-head" title={s ? `${[s.cmd, displayArgs(s.args)].filter(Boolean).join(' ')} · ${s.cwd} · ${s.id}` : ''}>
                  <span class="pane-name">{s ? sessionTitle(s) : t('panes.empty')}</span>
                  {#if s?.name}<span class="pane-profile">{s.profileId}</span>{/if}
                  {#if s?.model}<span class="pane-profile" title={s.model}>{prettyModel(s.model)}</span>{/if}
                  {#if s && needsAttention(s)}<span class="attn {s.attentionKind}">{attentionLabel(s.attentionKind)}</span>{/if}
                  <button
                    class="pane-close"
                    title={t('panes.close')}
                    aria-label={t('panes.close')}
                    onmousedown={(e) => e.stopPropagation()}
                    onclick={() => closePane(p.leaf.id)}>×</button
                  >
                </div>
              {/if}
              <div class="pane-body" style:background={termBg}>
                {#if prog}<div class="pane-progress"><ProgressBar progress={prog} thick /></div>{/if}
                {#if p.leaf.session}
                  {#key p.leaf.session}
                    <div class="term-slot" {@attach termSlot(p.leaf.session)}></div>
                  {/key}
                {:else if multi}
                  <div class="pane-empty">{t('panes.emptyHint')}</div>
                {/if}
                {#if focused && selected && view === 'changes'}
                  <Changes
                    sessionId={selected.id}
                    live={selected.state !== 'exited'}
                    sharedWith={inProjectFolder(selected) ? folderSharers(sessions, selected.projectId, selected.id).map(sessionTitle) : []}
                    onMoveToWorktree={selected.state !== 'exited' && selected.profileId !== SHELL_PROFILE
                      ? () => selected && openHandoff(selected.id, { profileId: selected.profileId, where: 'worktree' })
                      : undefined}
                    gitStamp={selected.git ? `${selected.git.head ?? ''}|${selected.git.files}|${selected.git.uncommitted}|${selected.git.ahead}` : ''}
                    prStamp={selected.git ? `${selected.git.head ?? ''}|${selected.git.ahead}` : ''}
                    onError={showError}
                    onRemoved={() => {
                      setView('terminal')
                      void refreshSessions()
                    }}
                  />
                {/if}
                {#if focused && selected && view === 'messages'}
                  <Messages
                    session={selected}
                    {sessions}
                    projectName={projects.find((x) => x.id === selected.projectId)?.name ?? ''}
                    stamp={msgStamps[selected.id] ?? 0}
                    onError={showError}
                  />
                {/if}
                {#if focused && search.open && search.session === p.leaf.session}
                  <!-- svelte-ignore a11y_no_static_element_interactions -->
                  <div class="search" onmousedown={(e) => e.stopPropagation()}>
                    <input
                      bind:this={searchInput}
                      bind:value={search.query}
                      placeholder={t('search.placeholder')}
                      spellcheck="false"
                      oninput={() => runSearch(false, true)}
                      onkeydown={onSearchKey}
                    />
                    <span class="search-status">{searchStatus}</span>
                    <button
                      class="search-btn"
                      class:on={search.caseSensitive}
                      title={t('search.caseSensitive')}
                      aria-pressed={search.caseSensitive}
                      onclick={() => {
                        search.caseSensitive = !search.caseSensitive
                        // The addon keeps its highlights for the same term:
                        // drop them so the matches are recounted.
                        terms.clearSearch(search.session)
                        runSearch(false)
                        searchInput?.focus()
                      }}>Aa</button
                    >
                    <button class="search-btn" title={t('search.prev')} aria-label={t('search.prev')} onclick={() => runSearch(true)}>↑</button>
                    <button class="search-btn" title={t('search.next')} aria-label={t('search.next')} onclick={() => runSearch(false)}>↓</button>
                    <button class="search-btn" title={t('search.close')} aria-label={t('search.close')} onclick={() => closeSearch()}>×</button>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/each}
        {#each dividers as d (d.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="divider {d.dir}"
            class:active={dragging === d.id}
            style={dividerStyle(d)}
            onpointerdown={(e) => startDrag(e, d)}
          ></div>
        {/each}
      </div>
      <div class="parking" bind:this={parkingEl}></div>

      {#if loaded && !selected && !multi}
        <div class="empty">
          {#if projects.length === 0}
            <Welcome
              {profiles}
              onAddProject={addProject}
              onOpenSettings={() => openSettings('agents')}
              onRescan={async () => {
                const ps = await guard(api.rescanProfiles())
                if (ps) profiles = ps
              }}
            />
          {:else}
            <h2>{t('empty.noSessions')}</h2>
            <p>
              {@html spawnable[0]
                ? th('empty.launchHintKey', { name: spawnable[0].name || spawnable[0].id })
                : th('empty.launchHint')}
            </p>
            {#if currentProjectId && spawnable.length}
              <div class="quick">
                {#each spawnable as prof (prof.id)}
                  <button onclick={() => spawn(currentProjectId!, prof.id)}>{prof.name || prof.id}</button>
                {/each}
              </div>
            {/if}
            <button class="linkish" onclick={() => openSettings('agents')}>{t('empty.chooseAgents')} <kbd>{keyLabel('⌘,')}</kbd></button>
          {/if}
        </div>
      {/if}
    </div>

    {#if gridOpen && !settingsOpen}
      <Grid
        {projects}
        {sessions}
        projectId={gridProjectId ?? currentProjectId}
        {selectedId}
        {now}
        onOpen={openFromGrid}
        onClose={closeGrid}
      />
    {/if}

    {#if orphanView && !settingsOpen}
      <div class="orphan-view">
        {#key orphanView.id}
          <Changes
            worktreeId={orphanView.id}
            startRemove={orphanView.remove}
            onError={showError}
            onClose={closeOrphan}
            onRemoved={() => {
              const pid = Object.keys(worktrees).find((k) => worktrees[k].some((w) => w.id === orphanView?.id))
              closeOrphan()
              if (pid) void refreshWorktrees(pid)
            }}
          />
        {/key}
      </div>
    {/if}

    {#if convoProject && !settingsOpen && !historyOpen}
      {@const cp = projects.find((p) => p.id === convoProject)}
      {#if cp}
        <Conversation
          project={cp}
          {sessions}
          defaultAuto={defaultAutoMessages}
          stamp={convoStamp}
          onUpdateProject={(patch) => updateProject(cp.id, patch)}
          onSelect={(id) => {
            convoProject = null
            if (sessions.some((s) => s.id === id)) select(id)
          }}
          onClose={() => {
            convoProject = null
            terms.focus()
          }}
        />
      {/if}
    {/if}

    {#if historyOpen && !settingsOpen}
      <History
        {projects}
        projectId={currentProjectId}
        stamp={historyStamp}
        onResume={resumeHistory}
        onClose={closeHistory}
      />
    {/if}

    {#if settingsOpen}
      <Settings
        initialTab={settingsTab}
        {profiles}
        {projects}
        projectId={currentProjectId}
        onProfiles={(ps) => (profiles = ps)}
        onSettings={(st) => {
          defaultMode = st.git?.defaultMode === 'same' ? 'same' : 'worktree'
          defaultAutoMessages = st.messages?.autoDeliver ?? true
        }}
        onInstall={(id) => void installAgent(id)}
        onClose={closeSettings}
      />
    {/if}

    {#if daemonDown}
      <div class="banner">{t('banner.reconnecting')} <span>{daemonDown}</span></div>
    {:else if daemonOld}
      <div class="banner action">
        {#if confirmRestart && busyCount > 0}
          {t('banner.restartBusy', { n: busyCount })}
          <button onclick={() => restartDaemon(true)}>{t('banner.restartAnyway')}</button>
          <button onclick={() => (confirmRestart = false)}>{t('banner.cancel')}</button>
        {:else if daemonOld.newer}
          {t('banner.daemonNewer', { version: daemonOld.daemonVersion })}
        {:else}
          {daemonOld.moved
            ? t('banner.daemonMoved')
            : daemonOld.daemonVersion
              ? t('banner.daemonOld', { version: daemonOld.daemonVersion })
              : t('banner.daemonOldUnknown')}
          <button disabled={restartingDaemon} onclick={() => restartDaemon()}>
            {restartingDaemon ? t('banner.restarting') : t('banner.restartNow')}
          </button>
        {/if}
      </div>
    {:else if updateBannerVisible()}
      {@const u = availableUpdate()!}
      <div class="banner action">
        {#if u.ready}
          {t('banner.updateReady', { version: u.version })}
          <button disabled={updateDownloading()} onclick={() => void guard(updateNow())}>
            {updateDownloading() ? t('banner.updateDownloadingButton') : t('banner.updateRestart')}
          </button>
        {:else}
          {updateDownloading()
            ? t('banner.updateDownloading', { version: u.version })
            : t('banner.updateAvailable', { version: u.version })}
          <button disabled={updateDownloading()} onclick={() => void guard(updateNow())}>
            {updateDownloading() ? t('banner.updateDownloadingButton') : u.url ? t('banner.download') : t('banner.update')}
          </button>
        {/if}
        <button onclick={dismissUpdate}>{t('banner.later')}</button>
      </div>
    {/if}
    {#if copyOffer && selected?.id === copyOffer.sessionId}
      <div class="banner action info">
        {#if copyOffer.done}
          {copyOffer.done}
        {:else}
          {t('setup.offer', { files: copyOffer.files.join(', ') })}
          <button disabled={copyOffer.busy} onclick={() => void acceptCopy(false)}>{t('setup.copyOnce')}</button>
          <button disabled={copyOffer.busy} onclick={() => void acceptCopy(true)}>{t('setup.copySave')}</button>
          <button onclick={() => (copyOffer = null)}>{t('setup.dismiss')}</button>
        {/if}
      </div>
    {/if}
    {#if handoffFor}
      {@const ps = paneSize()}
      <Handoff
        session={handoffFor.session}
        {sessions}
        {profiles}
        preset={handoffFor.preset}
        cols={ps.cols}
        rows={ps.rows}
        onDone={handedOff}
        onClose={() => (handoffFor = null)}
      />
    {/if}
    {#if setupAsk}
      <TrustSetup command={setupAsk.command} file={setupAsk.file} onChoose={setupAsk.resolve} />
    {/if}
    {#if error}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="toast" onclick={() => (error = null)}>{error}</div>
    {/if}
  </main>
</div>

<style>
  .layout {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }
  .main {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg);
  }
  .titlebar {
    height: 38px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 14px;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
  }
  .title {
    font-weight: 600;
    color: var(--fg);
  }
  .title-badge {
    display: flex;
    margin-right: -4px;
  }
  .profile {
    color: var(--muted);
    font-size: 11.5px;
  }
  .attn {
    flex: none;
    max-width: 45%;
    overflow: hidden;
    text-overflow: ellipsis;
    padding: 1px 8px;
    border-radius: 10px;
    font-size: 11px;
    color: var(--amber);
    background: color-mix(in srgb, var(--amber) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--amber) 35%, transparent);
  }
  .attn.done {
    color: var(--green);
    background: color-mix(in srgb, var(--green) 10%, transparent);
    border-color: color-mix(in srgb, var(--green) 30%, transparent);
  }
  .attn.error {
    color: var(--red);
    background: color-mix(in srgb, var(--red) 10%, transparent);
    border-color: color-mix(in srgb, var(--red) 30%, transparent);
  }
  .grow {
    flex: 1;
  }
  .activity {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .timer {
    flex: none;
    color: var(--muted);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }
  .timer.stalled {
    color: var(--orange);
    font-weight: 600;
  }
  .branch {
    flex: none;
    font: 11.5px var(--mono);
    color: var(--blue);
  }
  .port {
    flex: none;
    font: 11.5px var(--mono);
    color: var(--teal);
  }
  .perm {
    flex: none;
    padding: 1px 7px;
    border-radius: 10px;
    font-size: 11px;
    color: var(--muted);
    background: var(--hover);
  }
  .perm.bypass {
    color: var(--amber);
    background: color-mix(in srgb, var(--amber) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--amber) 35%, transparent);
  }
  .branch.same {
    color: var(--fg-dim);
  }
  .view-tabs {
    flex: none;
    display: flex;
    gap: 2px;
    padding: 0 10px;
    border-bottom: 1px solid var(--border);
  }
  .view-tabs button {
    padding: 5px 10px 4px;
    font-size: 12px;
    color: var(--muted);
    border-bottom: 2px solid transparent;
  }
  .view-tabs button:hover {
    color: var(--fg);
  }
  .view-tabs button.on {
    color: var(--fg);
    border-bottom-color: var(--blue);
  }
  .orphan-view {
    position: absolute;
    inset: 38px 0 0 0;
    z-index: 5;
    background: var(--bg);
  }
  .subtitle {
    color: var(--muted);
    font-family: var(--mono);
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .term-area {
    position: relative;
    flex: 1;
    min-height: 0;
  }

  /* Split panes: absolutely placed by their rect in the tree. */
  .panes {
    position: absolute;
    inset: 0;
  }
  .panes.dragging {
    user-select: none;
  }
  .panes.dragging .pane {
    pointer-events: none;
  }
  .pane {
    position: absolute;
    display: flex;
  }
  .multi .pane {
    padding: 3px;
  }
  .pane-frame {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    border-radius: 4px;
    overflow: hidden;
  }
  .multi .pane-frame {
    border: 1px solid var(--border);
  }
  .multi .pane.focused .pane-frame {
    border-color: color-mix(in srgb, var(--blue) 70%, transparent);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--blue) 35%, transparent);
  }
  .pane-head {
    flex: none;
    height: 22px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 4px 0 8px;
    font-size: 11.5px;
    color: var(--muted);
    background: var(--sidebar-bg);
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
  }
  .pane.focused .pane-head {
    color: var(--fg);
  }
  .pane-name {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pane-profile {
    color: var(--muted);
  }
  .pane-head .attn {
    padding: 0 6px;
    font-size: 10.5px;
  }
  .pane-close {
    margin-left: auto;
    width: 18px;
    height: 18px;
    border-radius: 4px;
    line-height: 1;
    font-size: 14px;
    color: var(--muted);
  }
  .pane-close:hover {
    background: var(--hover-strong);
    color: var(--fg);
  }
  .pane-body {
    position: relative;
    flex: 1;
    min-height: 0;
  }
  .term-slot {
    position: absolute;
    inset: 4px 0 0 8px;
  }
  .multi .term-slot {
    inset: 3px 0 0 6px;
  }
  .term-slot :global(.term-instance),
  .parking :global(.term-instance) {
    position: absolute;
    inset: 0;
  }
  .parking {
    display: none;
  }
  .pane-progress {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 2;
  }
  .pane-empty {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 12px;
    text-align: center;
    color: var(--muted);
    font-size: 12px;
  }
  .divider {
    position: absolute;
    z-index: 4;
  }
  .divider.row {
    width: 8px;
    margin-left: -4px;
    cursor: col-resize;
  }
  .divider.col {
    height: 8px;
    margin-top: -4px;
    cursor: row-resize;
  }
  .divider:hover,
  .divider.active {
    background: color-mix(in srgb, var(--blue) 30%, transparent);
  }

  /* ⌘F search bar, top right of the focused pane. */
  .search {
    position: absolute;
    top: 6px;
    right: 14px;
    z-index: 6;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 4px 4px 6px;
    border-radius: 6px;
    background: var(--menu-bg);
    border: 1px solid var(--border-strong);
    box-shadow: 0 6px 18px var(--shadow);
  }
  .search input {
    width: 190px;
    padding: 3px 6px;
    font: 12px var(--mono);
    color: var(--fg);
    background: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    outline: none;
  }
  .search input:focus {
    border-color: var(--blue);
  }
  .search-status {
    min-width: 54px;
    color: var(--muted);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    text-align: right;
  }
  .search-btn {
    min-width: 22px;
    height: 22px;
    padding: 0 4px;
    border-radius: 4px;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .search-btn:hover {
    background: var(--hover-strong);
    color: var(--fg);
  }
  .search-btn.on {
    background: var(--accent-bg);
    color: var(--fg);
  }

  .empty {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    background: var(--bg);
    color: var(--fg-dim);
    text-align: center;
  }
  .empty h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: var(--fg);
  }
  .empty p {
    margin: 0 0 10px;
    max-width: 340px;
  }
  /* :global: also styles the markup of {@html th(...)} strings. */
  .empty :global(kbd) {
    font-family: var(--mono);
    font-size: 11px;
    padding: 1px 5px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
  }
  .primary,
  .quick button {
    padding: 6px 14px;
    border-radius: 6px;
    border: 1px solid var(--border-strong);
    color: var(--fg);
    background: var(--hover);
  }
  .primary:hover,
  .quick button:hover {
    background: var(--accent-bg);
  }
  .linkish {
    margin-top: 14px;
    color: var(--muted);
    font-size: 12px;
  }
  .linkish:hover {
    color: var(--fg);
  }
  .quick {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 6px;
  }
  .banner {
    position: absolute;
    top: 46px;
    left: 50%;
    transform: translateX(-50%);
    padding: 6px 12px;
    z-index: 30; /* above Changes, the grid, the orphan view and Settings */
    border-radius: 6px;
    background: var(--warn-bg);
    color: var(--amber);
    font-size: 12px;
  }
  .banner span {
    color: var(--muted);
    margin-left: 6px;
  }
  .banner.action {
    display: flex;
    align-items: center;
    gap: 10px;
    max-width: 70%;
  }
  .banner.action button {
    flex: none;
    padding: 3px 10px;
    border: 1px solid var(--amber);
    border-radius: 5px;
    background: transparent;
    color: var(--amber);
    font: inherit;
    cursor: pointer;
  }
  .banner.action button:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .banner.info {
    top: 84px;
  }
  .toast {
    position: absolute;
    right: 14px;
    bottom: 14px;
    z-index: 30;
    max-width: 420px;
    padding: 8px 12px;
    border-radius: 6px;
    background: var(--danger-bg);
    color: var(--danger-fg);
    border: 1px solid var(--danger-border);
    font-size: 12px;
    cursor: default;
  }
</style>
