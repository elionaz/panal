<script lang="ts">
  // "Changes" view: a session's (or an orphaned worktree's) diff — file list,
  // commits ahead of the base, the selected file rendered with diff2html —
  // and the worktree actions (editor, Finder, copy branch, merge, pull
  // request, remove).
  import { onMount } from 'svelte'
  import { html as renderDiff } from 'diff2html'
  import type { ColorSchemeType } from 'diff2html/lib/types'
  import 'diff2html/bundles/css/diff2html.min.css'
  import { BrowserOpenURL, ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import { api, errorMessage, type DiffFile, type DiffResult, type ForgeListItem, type LossInfo, type PRResult } from './api'
  import { hasKey, locale, t, th } from './i18n.svelte'
  import PullRequest from './PullRequest.svelte'
  import { groupPRs, selectedBranch, showPRList } from './sessionprs'
  import { shortBranch } from './branch'
  import { joinNames } from './shared'

  interface Props {
    /** What to diff: a session, or a worktree without one. */
    sessionId?: string
    worktreeId?: string
    /** A live session uses the worktree (it cannot be removed). */
    live?: boolean
    /** The other live sessions working in the same project folder. */
    sharedWith?: string[]
    /** Continue this session in a worktree of its own (shared folder). */
    onMoveToWorktree?: () => void
    /** Changes whenever the daemon reports new git info: reload. */
    gitStamp?: string
    /** Changes when the branch moves (checkout, commits ahead): reloads the
     * pull request block, which talks to the host (gitStamp changes on every
     * edited file). */
    prStamp?: string
    /** Open the remove confirmation right away. */
    startRemove?: boolean
    onError: (msg: string) => void
    /** The worktree was removed. */
    onRemoved?: () => void
    /** Shown as a close button (orphan overlay). */
    onClose?: () => void
  }

  let { sessionId, worktreeId, live = false, sharedWith = [], onMoveToWorktree, gitStamp, prStamp, startRemove = false, onError, onRemoved, onClose }: Props = $props()

  const STYLE_KEY = 'panal.diffStyle'
  /** Below this width (px) the automatic style is unified: two columns
   * would cut most lines. */
  const NARROW_DIFF = 900
  type DiffStyle = 'side-by-side' | 'line-by-line'
  let diff = $state<DiffResult | null>(null)
  let loading = $state(false)
  let loadError = $state<string | null>(null)
  let selectedPath = $state<string | null>(null)
  /** The user's choice; null: automatic (unified when the diff is narrow). */
  let stylePref = $state<DiffStyle | null>(readStyle())
  let diffWidth = $state(0)
  const style = $derived<DiffStyle>(stylePref ?? (diffWidth > 0 && diffWidth < NARROW_DIFF ? 'line-by-line' : 'side-by-side'))
  let menuOpen = $state(false)
  let commitsOpen = $state(true)
  let prBlock = $state<ReturnType<typeof PullRequest> | null>(null)
  let hasPR = $state(false)
  let prCreating = $state(false)
  // The pull requests of every branch the session worked on (forge.list).
  let prList = $state<ForgeListItem[] | null>(null)
  let prPicked = $state('') // a listed other branch shown in the PR block
  let prListOpen = $state(true)
  /** The merged or closed ones, folded by default. */
  let prDoneOpen = $state(false)
  let prListLoading = $state(false)
  const prGroups = $derived(groupPRs(prList))
  const prSummary = $derived(
    [
      prGroups.open.length ? t('changes.prsOpen', { n: prGroups.open.length }) : '',
      prGroups.done.length ? t('changes.prsDone', { n: prGroups.done.length }) : '',
    ]
      .filter(Boolean)
      .join(' · '),
  )
  let prListSeq = 0
  const prBranch = $derived(selectedBranch(prList, prPicked))
  let notice = $state<string | null>(null)

  // Remove confirmation.
  let removing = $state(false)
  let deleteBranch = $state(false)
  let force = $state(false)
  let loss = $state<LossInfo | null>(null)
  let lossLoading = $state(false)
  let busy = $state(false)
  // Merge confirmation, and the offer to remove the worktree after a merge.
  let merging = $state(false)
  let removeOffer = $state(false)
  /** The merge result, shown in the remove offer (not as a timed notice, whose
   * disappearing would move the offer's buttons under the pointer). */
  let mergedMsg = $state('')
  // Pull request confirmation: remote and URL from a dry run.
  let prOpen = $state(false)
  let prInfo = $state<PRResult | null>(null)
  let prLoading = $state(false)
  let prCheckError = $state<string | null>(null)
  let prSeq = 0 // drops stale dry-run answers
  /** gh / glab work for this branch: the PR block handles pull requests. */
  let forgeAvailable = $state(false)
  /** ⟳ also reloads the PR block. */
  let refreshTick = $state(0)
  // No block, no PR flow of its own: the push-and-browse button is back.
  $effect(() => {
    if (!diff?.branch) forgeAvailable = false
  })

  const wtId = $derived(diff?.worktreeId || worktreeId || '')
  const isWorktree = $derived(diff?.mode === 'worktree' || !!worktreeId)
  const selected = $derived(diff?.files.find((f) => f.path === selectedPath) ?? diff?.files[0] ?? null)
  const rendered = $derived.by(() => {
    if (!selected?.patch) return ''
    try {
      return renderDiff(selected.patch, {
        outputFormat: style,
        drawFileList: false,
        matching: 'lines',
        colorScheme: 'dark' as ColorSchemeType,
      })
    } catch (e) {
      return `<pre class="raw">${escapeHtml(selected.patch)}</pre>`
    }
  })

  function readStyle(): DiffStyle | null {
    try {
      const v = localStorage.getItem(STYLE_KEY)
      return v === 'line-by-line' || v === 'side-by-side' ? v : null
    } catch {
      return null
    }
  }

  function setStyle(s: DiffStyle) {
    stylePref = s
    try {
      localStorage.setItem(STYLE_KEY, s)
    } catch {
      /* ignore */
    }
  }

  function escapeHtml(s: string): string {
    return s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!)
  }

  let seq = 0
  async function load() {
    const my = ++seq
    loading = true
    try {
      const d = sessionId ? await api.sessionDiff(sessionId) : await api.worktreeDiff(worktreeId!)
      if (my !== seq) return
      diff = d
      loadError = null
      if (!d.files.some((f) => f.path === selectedPath)) selectedPath = d.files[0]?.path ?? null
    } catch (e) {
      if (my === seq) loadError = errorMessage(e)
    } finally {
      if (my === seq) loading = false
    }
  }

  // Reload when the target changes or the daemon reports new git info.
  let lastKey = ''
  $effect(() => {
    const key = `${sessionId ?? ''}|${worktreeId ?? ''}|${gitStamp ?? ''}`
    if (key === lastKey) return
    const targetChanged = lastKey.split('|').slice(0, 2).join('|') !== key.split('|').slice(0, 2).join('|')
    lastKey = key
    if (targetChanged) {
      diff = null
      selectedPath = null
      removing = merging = prOpen = removeOffer = false
      prSeq++
      prLoading = false
    }
    void load()
  })

  function statusClass(f: DiffFile): string {
    return `st st-${f.status}`
  }

  function fileName(path: string): { dir: string; base: string } {
    const i = path.lastIndexOf('/')
    return i < 0 ? { dir: '', base: path } : { dir: path.slice(0, i + 1), base: path.slice(i + 1) }
  }

  function flash(msg: string) {
    notice = msg
    window.setTimeout(() => notice === msg && (notice = null), 4000)
  }

  async function open(target: 'editor' | 'finder') {
    try {
      await api.openFolder(target, wtId ? { worktreeId: wtId } : { sessionId })
    } catch (e) {
      onError(errorMessage(e))
    }
  }

  async function copyBranch() {
    if (!diff?.branch) return
    await ClipboardSetText(diff.branch)
    flash(t('changes.copied', { branch: diff.branch }))
  }

  /** Create pull request: the host's form when gh / glab work here, else
   * push the branch and open the host's page. */
  function createPR() {
    if (forgeAvailable && prBlock) {
      merging = removing = prOpen = removeOffer = false
      prBlock.openCreate()
    } else void togglePR()
  }

  async function loadPRList(refresh = false) {
    const my = ++prListSeq
    prListLoading = true
    try {
      const l = await api.forgeList(sessionId ?? '', worktreeId ?? '', refresh)
      if (my === prListSeq) prList = l.prs ?? []
    } catch {
      if (my === prListSeq) prList = null // no list: the PR block alone, as before
    } finally {
      if (my === prListSeq) prListLoading = false
    }
  }
  // Reload when the target or its branch moves; the daemon caches a minute.
  let prListKey = ''
  $effect(() => {
    const key = `${sessionId ?? ''}|${worktreeId ?? ''}|${prStamp ?? ''}|${diff?.branch ?? ''}`
    if (!diff?.branch || key === prListKey) return
    if (prListKey.split('|').slice(0, 2).join('|') !== key.split('|').slice(0, 2).join('|')) prPicked = ''
    prListKey = key
    void loadPRList()
  })

  /** A PR state as the PR block names it ("open", "merged"…). */
  function prState(state: string): string {
    const k = `pr.state.${state}`
    return hasKey(k) ? t(k) : state
  }

  function menu(action: () => void) {
    menuOpen = false
    action()
  }

  function toggleMerge() {
    merging = !merging
    removing = prOpen = removeOffer = false
  }

  async function doMerge() {
    if (!wtId) return
    busy = true
    try {
      const r = await api.mergeWorktree(wtId)
      merging = false
      let msg = t('changes.merged', { branch: r.branch, base: r.base, n: r.commits })
      if (r.uncommitted) msg += t('changes.mergedUncommitted', { n: r.uncommitted })
      if (live) flash(msg)
      else ((mergedMsg = msg), (removeOffer = true))
      void load()
    } catch (e) {
      onError(errorMessage(e))
    } finally {
      busy = false
    }
  }

  async function togglePR() {
    if (prOpen) {
      prOpen = false
      prSeq++
      prLoading = false
      return
    }
    merging = removing = removeOffer = false
    prOpen = true
    prInfo = null
    prCheckError = null
    if (!wtId || !diff?.git.ahead) return
    const my = ++prSeq
    const id = wtId
    prLoading = true
    try {
      const r = await api.pullRequest(id, true)
      if (my === prSeq && id === wtId) prInfo = r
    } catch (e) {
      if (my === prSeq && id === wtId) prCheckError = errorMessage(e)
    } finally {
      if (my === prSeq && id === wtId) prLoading = false
    }
  }

  async function doPR() {
    if (!wtId) return
    busy = true
    try {
      const r = await api.pullRequest(wtId)
      prOpen = false
      let msg = t(r.url ? 'changes.prOpened' : 'changes.prPushed', { branch: r.branch, remote: r.remote })
      if (r.uncommitted) msg += t('changes.prPushedUncommitted', { n: r.uncommitted })
      flash(msg)
      if (r.url) BrowserOpenURL(r.url)
      void load()
    } catch (e) {
      onError(errorMessage(e))
    } finally {
      busy = false
    }
  }

  /** After a merge: the remove confirmation, with the branch preselected. */
  function offerRemove() {
    askRemove(true)
  }

  async function checkLoss() {
    if (!wtId) return
    lossLoading = true
    try {
      const r = await api.removeWorktree(wtId, { deleteBranch, dryRun: true })
      loss = r.loss ?? null
    } catch (e) {
      onError(errorMessage(e))
    } finally {
      lossLoading = false
    }
  }

  function askRemove(withBranch = false) {
    deleteBranch = withBranch
    merging = prOpen = removeOffer = false
    removing = true
    force = false
    void checkLoss()
  }

  async function doRemove() {
    if (!wtId) return
    busy = true
    try {
      await api.removeWorktree(wtId, { deleteBranch, force: force || !!loss })
      removing = false
      onRemoved?.()
    } catch (e) {
      onError(errorMessage(e))
      void checkLoss()
    } finally {
      busy = false
    }
  }

  onMount(() => {
    if (startRemove) askRemove()
  })
</script>

<svelte:window
  onclick={() => (menuOpen = false)}
  onkeydown={(e) => {
    if (e.key === 'Escape') menuOpen = false
  }}
/>

<div class="changes">
  <header class="bar">
    <div class="info">
      {#if diff?.branch}
        <span class="branch" title={diff.dir}>⎇ {diff.branch}{#if isWorktree && diff.base}<span class="base">{` → ${diff.base}`}</span>{/if}</span>
        {#if diff.startBranch}
          <span class="started" title={(diff.branches ?? []).join('\n')}>{t('changes.startedOn', { branch: shortBranch(diff.startBranch) })}</span>
        {/if}
      {/if}
      {#if diff}
        <span class="stats">
          {t('changes.files', { n: diff.files.length })}
          <span class="add">+{diff.additions}</span>
          <span class="del">−{diff.deletions}</span>
          {#if diff.git.ahead}<span class="ahead">· {t('changes.ahead', { n: diff.git.ahead })}</span>{/if}
        </span>
      {/if}
      <button class="icon" class:spin={loading} title={t('changes.refresh')} aria-label={t('changes.refresh')} onclick={() => {
          refreshTick++
          void load()
          if (diff?.branch) void loadPRList(true)
        }}>⟳</button>
    </div>
    <span class="actions">
      {#if !prBranch && ((forgeAvailable && !hasPR) || (!forgeAvailable && isWorktree && wtId))}
        <button
          class="btn primary"
          class:on={prCreating || prOpen}
          disabled={busy || (!forgeAvailable && !diff?.git.ahead)}
          title={!forgeAvailable && !diff?.git.ahead ? t('pr.nothingToCreate') : ''}
          onclick={createPR}>{t('changes.createPR')}</button
        >
      {/if}
      {#if isWorktree && wtId}
        <button class="btn" class:on={merging} disabled={busy} onclick={toggleMerge}>{t('changes.mergeInto', { base: diff?.base || 'base' })}</button>
      {/if}
      <span class="more">
        <button
          class="btn icon-btn"
          class:on={menuOpen}
          title={t('changes.more')}
          aria-label={t('changes.more')}
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onclick={(e) => {
            e.stopPropagation()
            menuOpen = !menuOpen
          }}>⋯</button
        >
        {#if menuOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div class="menu" role="menu" tabindex="-1" onclick={(e) => e.stopPropagation()}>
            <button class="menu-item" role="menuitem" onclick={() => menu(() => open('editor'))}>{t('changes.openEditor')}</button>
            <button class="menu-item" role="menuitem" onclick={() => menu(() => open('finder'))}>{t('changes.openFinder')}</button>
            {#if diff?.branch}<button class="menu-item" role="menuitem" onclick={() => menu(copyBranch)}>{t('changes.copyBranch')}</button>{/if}
            {#if isWorktree && wtId}
              <div class="menu-sep"></div>
              <button
                class="menu-item danger"
                role="menuitem"
                disabled={busy || live}
                title={live ? t('changes.stopFirst') : ''}
                onclick={() => menu(() => askRemove())}>{t('changes.removeWorktree')}</button
              >
            {/if}
          </div>
        {/if}
      </span>
      {#if onClose}<button class="icon close" title={t('changes.close')} aria-label={t('changes.closeLabel')} onclick={onClose}>×</button>{/if}
    </span>
  </header>

  {#if sharedWith.length && !isWorktree}
    <div class="notice">
      {t('changes.shared', { n: sharedWith.length, names: joinNames(sharedWith) })}
      {#if onMoveToWorktree}<button class="notice-action" onclick={onMoveToWorktree}>{t('changes.moveToWorktree')}</button>{/if}
    </div>
  {/if}
  {#snippet prItem(it: ForgeListItem)}
    <li>
      <button
        class="pritem"
        class:sel={prBranch ? prBranch === it.branch : it.current}
        title={it.branch}
        onclick={() => (prPicked = it.current ? '' : it.branch)}
      >
        <span class="num">#{it.pr?.number}</span>
        <span class="ptitle">{it.pr?.title}</span>
        <span class="pstate st-{it.pr?.state}">{prState(it.pr?.state ?? '')}</span>
        {#if it.pr?.checks?.state === 'passing'}<span class="ck ck-passing">✓</span>
        {:else if it.pr?.checks?.state === 'failing'}<span class="ck ck-failing">✗</span>
        {:else if it.pr?.checks?.state === 'pending'}<span class="ck ck-pending">●</span>{/if}
        {#if it.current}<span class="cur">{t('changes.currentBranch')}</span>{/if}
      </button>
    </li>
  {/snippet}
  {#if diff?.branch && prList === null && prListLoading}
    <div class="prlist"><p class="section loading">{t('changes.prsLoading')}</p></div>
  {:else if diff?.branch && showPRList(prList)}
    <div class="prlist">
      <button class="section toggle" aria-expanded={prListOpen} onclick={() => (prListOpen = !prListOpen)}
        ><span class="caret" class:open={prListOpen}>▸</span> {t('changes.sessionPRs', { n: prList?.length ?? 0 })}
        <span class="prsum">{prSummary}</span></button
      >
      {#if prListOpen}
        {#if prGroups.open.length}
          <ul>
            {#each prGroups.open as it (it.branch)}{@render prItem(it)}{/each}
          </ul>
        {/if}
        {#if prGroups.done.length}
          <button class="section toggle sub" aria-expanded={prDoneOpen} onclick={() => (prDoneOpen = !prDoneOpen)}
            ><span class="caret" class:open={prDoneOpen}>▸</span> {t('changes.prsDone', { n: prGroups.done.length })}</button
          >
          {#if prDoneOpen}
            <ul>
              {#each prGroups.done as it (it.branch)}{@render prItem(it)}{/each}
            </ul>
          {/if}
        {/if}
      {/if}
    </div>
  {/if}
  {#if diff?.branch}
    <PullRequest
      branch={prBranch}
      {sessionId}
      {worktreeId}
      reloadKey={`${prStamp ?? diff.branch}|${diff.branch}|${refreshTick}`}
      {onError}
      onSelectFile={(p) => {
        if (diff?.files.some((f) => f.path === p)) selectedPath = p
      }}
      onMerged={isWorktree
        ? (msg) => {
            mergedMsg = msg
            merging = prOpen = removing = false
            removeOffer = true
          }
        : undefined}
      bind:available={forgeAvailable}
      bind:hasPR
      bind:creating={prCreating}
      bind:this={prBlock}
    />
  {/if}

  {#if notice}<div class="notice ok">{notice}</div>{/if}

  {#if merging && diff}
    <div class="confirm">
      <p>
        {@html th('changes.mergeInfo', { branch: diff.branch ?? '', base: diff.base ?? '' })}
        {#if diff.git.uncommitted}<br /><span class="warn">{t('changes.mergeUncommitted', { n: diff.git.uncommitted })}</span>{/if}
      </p>
      <div class="row">
        <button class="btn primary" disabled={busy || !diff.git.ahead} onclick={doMerge}>
          {diff.git.ahead ? t('changes.doMerge', { n: diff.git.ahead }) : t('changes.nothingToMerge')}
        </button>
        <button class="btn" onclick={() => (merging = false)}>{t('changes.cancel')}</button>
      </div>
    </div>
  {/if}

  {#if removeOffer && !merging && !removing}
    <div class="confirm">
      {#if mergedMsg}<p class="ok">{mergedMsg}</p>{/if}
      <p>{t('changes.removeOffer')}</p>
      <div class="row">
        <button class="btn danger" disabled={busy || live} onclick={offerRemove}>{t('changes.removeOfferYes')}</button>
        <button class="btn" onclick={() => (removeOffer = false)}>{t('changes.removeOfferNo')}</button>
      </div>
    </div>
  {/if}

  {#if prOpen && diff}
    <div class="confirm">
      {#if prLoading}
        <p class="muted">{t('changes.prChecking')}</p>
      {:else if prCheckError}
        <p class="err">{prCheckError}</p>
      {:else if prInfo}
        <p>
          {@html th('changes.prInfo', { branch: prInfo.branch, remote: prInfo.remote, base: prInfo.base })}
          {#if !prInfo.url}<br /><span class="muted">{t('changes.prNoUrl', { remote: prInfo.remote })}</span>{/if}
          {#if prInfo.uncommitted}<br /><span class="warn">{t('changes.prUncommitted', { n: prInfo.uncommitted })}</span>{/if}
        </p>
      {/if}
      <div class="row">
        <button class="btn primary" disabled={busy || prLoading || !prInfo || !diff.git.ahead} onclick={doPR}>
          {diff.git.ahead ? t('changes.doPR', { n: diff.git.ahead }) : t('changes.nothingToPR')}
        </button>
        <button class="btn" onclick={() => (prOpen = false)}>{t('changes.cancel')}</button>
      </div>
    </div>
  {/if}

  {#if removing}
    <div class="confirm danger-zone">
      <p>{diff?.dir ? t('changes.removeInfoDir', { dir: diff.dir }) : t('changes.removeInfo')}</p>
      <label class="check"
        ><input type="checkbox" bind:checked={deleteBranch} onchange={checkLoss} /> {t('changes.deleteBranch', { branch: diff?.branch ?? '' })}</label
      >
      {#if lossLoading}
        <p class="muted">{t('changes.checkingLoss')}</p>
      {:else if loss}
        <div class="loss">
          {#if loss.uncommitted?.length}
            <b>{t('changes.lossFiles', { n: loss.uncommitted.length })}</b>
            <ul>
              {#each loss.uncommitted.slice(0, 12) as f (f)}<li><code>{f}</code></li>{/each}
              {#if loss.uncommitted.length > 12}<li>{t('changes.andMore', { n: loss.uncommitted.length - 12 })}</li>{/if}
            </ul>
          {/if}
          {#if loss.unmerged?.length}
            <b>{t('changes.lossCommits', { n: loss.unmerged.length, base: loss.base })}</b>
            <ul>
              {#each loss.unmerged.slice(0, 8) as c (c.hash)}<li><code>{c.short}</code> {c.subject}</li>{/each}
            </ul>
          {/if}
          <label class="check"><input type="checkbox" bind:checked={force} /> {t('changes.removeAnyway')}</label>
        </div>
      {:else}
        <p class="muted">{deleteBranch ? t('changes.nothingLost') : t('changes.nothingLostKeep')}</p>
      {/if}
      <div class="row">
        <button class="btn danger" disabled={busy || lossLoading || (!!loss && !force)} onclick={doRemove}>{t('changes.remove')}</button>
        <button class="btn" onclick={() => (removing = false)}>{t('changes.cancel')}</button>
      </div>
    </div>
  {/if}

  {#if loadError && !diff}
    <div class="empty">
      <p class="err">{loadError}</p>
      <button class="btn" onclick={load}>{t('changes.retry')}</button>
    </div>
  {:else if !diff}
    <div class="empty"><p class="muted">{t('changes.loading')}</p></div>
  {:else}
    {#if diff.truncated}
      <div class="notice">{t('changes.truncated')}</div>
    {/if}
    <div class="body">
      <aside class="files">
        {#if diff.commits.length}
          <button class="section toggle" aria-expanded={commitsOpen} onclick={() => (commitsOpen = !commitsOpen)}
            ><span class="caret" class:open={commitsOpen}>▸</span> {t('changes.commits', { n: diff.commits.length })}</button
          >
          {#if commitsOpen}<ul class="commits">
            {#each diff.commits as c (c.hash)}
              <li title={`${c.hash}\n${c.author} · ${new Date(c.date).toLocaleString(locale())}`}><code>{c.short}</code> {c.subject}</li>
            {/each}
          </ul>{/if}
        {/if}
        <div class="section">{t('changes.fileList', { n: diff.files.length })}</div>
        {#each diff.files as f (f.path)}
          {@const n = fileName(f.path)}
          <button class="file" class:sel={selected?.path === f.path} title={f.oldPath ? `${f.oldPath} → ${f.path}` : f.path} onclick={() => (selectedPath = f.path)}>
            <span class={statusClass(f)}>{f.status}</span>
            <span class="fname">
              <span class="fbase">{n.base}</span>
              {#if n.dir}<span class="fdir">{n.dir.replace(/\/$/, '')}</span>{/if}
            </span>
            {#if f.binary}<span class="tag">bin</span>
            {:else}<span class="nums"><span class="add">+{f.additions}</span> <span class="del">−{f.deletions}</span></span>{/if}
          </button>
        {:else}
          <p class="muted pad">{isWorktree ? t('changes.noChangesBase', { base: diff.base ?? '' }) : t('changes.noChanges')}</p>
        {/each}
      </aside>
      <section class="diff" class:wrap={style === 'line-by-line'} bind:clientWidth={diffWidth}>
        {#if selected}
          <div class="diff-head">
            <span class={statusClass(selected)}>{selected.status}</span>
            <span class="dpath" title={selected.oldPath ? `${selected.oldPath} → ${selected.path}` : selected.path}
              >{selected.oldPath ? `${selected.oldPath} → ` : ''}{selected.path}</span
            >
            {#if !selected.binary}<span class="nums"><span class="add">+{selected.additions}</span> <span class="del">−{selected.deletions}</span></span>{/if}
            <span class="seg" role="group" aria-label={t('changes.style')}>
              <button class:on={style === 'line-by-line'} title={t('changes.unified')} aria-label={t('changes.unified')} onclick={() => setStyle('line-by-line')}>≡</button>
              <button class:on={style === 'side-by-side'} title={t('changes.sideBySide')} aria-label={t('changes.sideBySide')} onclick={() => setStyle('side-by-side')}>◫</button>
            </span>
          </div>
          {#if selected.binary}
            <div class="empty"><p class="muted">{t('changes.binary', { path: selected.path })}</p></div>
          {:else if selected.truncated || !selected.patch}
            <div class="empty">
              <p class="muted">
                {selected.truncated ? t('changes.noContentLimit', { path: selected.path }) : t('changes.noContentMode', { path: selected.path })}
              </p>
            </div>
          {:else}
            {@html rendered}
          {/if}
        {/if}
      </section>
    </div>
  {/if}
</div>

<style>
  .changes {
    position: absolute;
    inset: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    min-height: 0;
  }
  .bar {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  .info {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .branch {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--fg);
  }
  .base {
    color: var(--muted);
  }
  .stats {
    flex: none;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .add {
    color: var(--green);
    font-variant-numeric: tabular-nums;
  }
  .del {
    color: var(--red);
    font-variant-numeric: tabular-nums;
  }
  .ahead {
    color: var(--muted);
  }
  .actions {
    flex: none;
    margin-left: auto;
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .more {
    position: relative;
  }
  .icon-btn {
    padding: 3px 8px;
    line-height: 1;
  }
  .menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    z-index: 10;
    min-width: 180px;
    padding: 4px;
    background: var(--menu-bg);
    border: 1px solid var(--border-strong);
    border-radius: 8px;
    box-shadow: 0 8px 24px var(--shadow);
  }
  .menu-item {
    display: block;
    width: 100%;
    padding: 6px 8px;
    border-radius: 5px;
    font-size: 12.5px;
    color: var(--fg);
    text-align: left;
  }
  .menu-item:hover:not(:disabled) {
    background: var(--accent-bg);
  }
  .menu-item:disabled {
    opacity: 0.45;
  }
  .menu-item.danger {
    color: var(--red);
  }
  .menu-sep {
    height: 1px;
    margin: 4px 2px;
    background: var(--border);
  }
  .icon {
    width: 22px;
    height: 22px;
    border-radius: 5px;
    color: var(--muted);
    font-size: 14px;
  }
  .icon:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .icon.spin {
    animation: spin 0.8s linear infinite;
  }
  .close {
    font-size: 16px;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .seg {
    display: inline-flex;
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    overflow: hidden;
  }
  .seg button {
    padding: 2px 8px;
    font-size: 11.5px;
    color: var(--fg-dim);
  }
  .seg button.on {
    background: var(--accent-bg);
    color: var(--fg);
  }
  .btn {
    padding: 3px 9px;
    border-radius: 6px;
    border: 1px solid var(--border-strong);
    font-size: 12px;
    color: var(--fg-dim);
  }
  .btn:hover:not(:disabled) {
    background: var(--hover);
    color: var(--fg);
  }
  .btn:disabled {
    opacity: 0.45;
  }
  .btn.primary {
    background: var(--accent-bg);
    color: var(--fg);
  }
  .btn.on {
    border-color: var(--blue);
    color: var(--fg);
  }
  .btn.danger {
    color: var(--red);
    border-color: color-mix(in srgb, var(--red) 40%, transparent);
  }
  .notice {
    flex: none;
    padding: 6px 12px;
    font-size: 12px;
    color: var(--amber);
    background: color-mix(in srgb, var(--amber) 8%, transparent);
    border-bottom: 1px solid var(--border);
  }
  .notice-action {
    margin-left: 8px;
    padding: 1px 8px;
    border-radius: 5px;
    border: 1px solid color-mix(in srgb, var(--amber) 50%, transparent);
    color: var(--amber);
    font-size: 11.5px;
  }
  .notice-action:hover {
    background: color-mix(in srgb, var(--amber) 16%, transparent);
  }
  .notice.ok {
    color: var(--green);
    background: color-mix(in srgb, var(--green) 8%, transparent);
  }
  .confirm .ok {
    color: var(--green);
  }
  .confirm {
    flex: none;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--sidebar-bg);
    font-size: 12.5px;
  }
  .confirm p {
    margin: 0 0 8px;
    color: var(--fg-dim);
  }
  .confirm .row {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }
  .danger-zone {
    box-shadow: inset 3px 0 0 var(--red);
  }
  .warn {
    color: var(--amber);
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--fg-dim);
    margin: 4px 0;
  }
  .check input {
    accent-color: var(--red);
  }
  .loss {
    margin-top: 6px;
    color: var(--fg);
  }
  .loss ul {
    margin: 4px 0 8px;
    padding-left: 18px;
    max-height: 140px;
    overflow-y: auto;
    color: var(--fg-dim);
  }
  .muted {
    color: var(--muted);
  }
  .pad {
    padding: 8px 12px;
  }
  .err {
    color: var(--red);
  }
  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 20px;
    text-align: center;
  }
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .files {
    flex: none;
    width: 280px;
    overflow-y: auto;
    border-right: 1px solid var(--border);
    padding-bottom: 8px;
  }
  .section.toggle {
    display: block;
    width: 100%;
    text-align: left;
  }
  .section.toggle:hover {
    color: var(--fg);
  }
  .caret {
    display: inline-block;
    transition: transform 0.12s;
  }
  .caret.open {
    transform: rotate(90deg);
  }
  .section {
    padding: 8px 12px 4px;
    font-size: 10.5px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .commits {
    list-style: none;
    margin: 0;
    padding: 0 12px 4px;
    font-size: 12px;
    color: var(--fg-dim);
  }
  .commits li {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 1px 0;
  }
  code,
  .confirm :global(code) {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--blue);
  }
  .file {
    display: flex;
    align-items: baseline;
    gap: 6px;
    width: 100%;
    padding: 4px 12px;
    text-align: left;
    font-size: 12px;
    color: var(--fg-dim);
  }
  .file:hover {
    background: var(--hover);
  }
  .file.sel {
    background: var(--accent-bg);
    color: var(--fg);
  }
  .fname {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    white-space: nowrap;
  }
  .fbase {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .fdir {
    overflow: hidden;
    text-overflow: ellipsis;
    /* The end of the folder is what tells files apart: cut its start. */
    direction: rtl;
    text-align: left;
    font-size: 10.5px;
    color: var(--muted);
  }
  .nums {
    flex: none;
    font-size: 11px;
  }
  .tag {
    font-size: 10px;
    color: var(--muted);
    border: 1px solid var(--border-strong);
    border-radius: 3px;
    padding: 0 3px;
  }
  .st {
    flex: none;
    width: 14px;
    font-family: var(--mono);
    font-weight: 700;
    font-size: 11px;
    text-align: center;
  }
  .st-A {
    color: var(--green);
  }
  .st-M,
  .st-T {
    color: var(--amber);
  }
  .st-D {
    color: var(--red);
  }
  .st-R,
  .st-C {
    color: var(--blue);
  }
  .diff {
    flex: 1;
    min-width: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
  }
  .started {
    flex: none;
    font-size: 11.5px;
    color: var(--muted);
  }
  .prlist {
    flex: none;
    border-bottom: 1px solid var(--border);
    padding-bottom: 4px;
  }
  .prsum {
    margin-left: 6px;
    text-transform: none;
    letter-spacing: normal;
    font-size: 11.5px;
    color: var(--fg-dim);
  }
  .section.sub {
    padding-left: 22px;
    padding-top: 4px;
    font-size: 12px;
  }
  .section.loading {
    margin: 0;
    font-size: 12px;
  }
  .prlist ul {
    list-style: none;
    margin: 0;
    padding: 0 6px;
    max-height: 168px;
    overflow-y: auto;
  }
  .pritem {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 3px 6px;
    border-radius: 5px;
    font-size: 12px;
    color: var(--fg-dim);
    text-align: left;
  }
  .pritem:hover {
    background: var(--hover);
  }
  .pritem.sel {
    background: var(--accent-bg);
    color: var(--fg);
  }
  .num {
    flex: none;
    font-weight: 600;
    color: var(--blue);
    font-variant-numeric: tabular-nums;
  }
  .ptitle {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pstate {
    flex: none;
    padding: 0 7px;
    border-radius: 10px;
    font-size: 11px;
    background: var(--hover);
  }
  .pstate.st-open {
    color: var(--green);
    background: color-mix(in srgb, var(--green) 14%, transparent);
  }
  .pstate.st-merged {
    color: var(--blue);
    background: color-mix(in srgb, var(--blue) 14%, transparent);
  }
  .pstate.st-closed {
    color: var(--red);
    background: color-mix(in srgb, var(--red) 12%, transparent);
  }
  .ck {
    flex: none;
    font-size: 11px;
  }
  .ck-passing {
    color: var(--green);
  }
  .ck-failing {
    color: var(--red);
  }
  .ck-pending {
    color: var(--amber);
  }
  .cur {
    flex: none;
    font-size: 10.5px;
    color: var(--muted);
  }
  .diff-head {
    position: sticky;
    top: 0;
    z-index: 2;
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: var(--sidebar-bg);
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .dpath {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
    text-align: left;
    font-family: var(--mono);
    color: var(--fg);
  }
  .diff-head .seg button {
    padding: 1px 8px;
    font-size: 13px;
  }
  /* Our head names the file: diff2html's own is redundant. */
  .diff :global(.d2h-file-header) {
    display: none;
  }
  /* Unified: long lines wrap instead of running off the edge (side by
   * side keeps scrolling: its two columns are separate tables, and wrapped
   * rows would no longer line up). */
  .diff.wrap :global(.d2h-code-line) {
    white-space: normal;
  }
  .diff.wrap :global(.d2h-code-line-ctn) {
    white-space: pre-wrap;
    word-break: break-all;
  }
  .diff :global(.raw) {
    margin: 0;
    padding: 10px;
    font-family: var(--mono);
    font-size: 12px;
  }
  /* diff2html, dark scheme tuned to the app palette. */
  .diff :global(.d2h-dark-color-scheme) {
    --d2h-dark-bg-color: var(--bg);
    --d2h-dark-color: var(--fg);
    --d2h-dark-file-header-bg-color: var(--sidebar-bg);
    --d2h-dark-file-header-border-color: var(--border);
    --d2h-dark-border-color: var(--border-strong);
    --d2h-dark-line-border-color: var(--border);
    --d2h-dark-dim-color: var(--muted);
    --d2h-dark-info-bg-color: color-mix(in srgb, var(--blue) 8%, transparent);
    --d2h-dark-ins-bg-color: color-mix(in srgb, var(--green) 12%, transparent);
    --d2h-dark-del-bg-color: color-mix(in srgb, var(--red) 12%, transparent);
    --d2h-dark-ins-highlight-bg-color: color-mix(in srgb, var(--green) 35%, transparent);
    --d2h-dark-del-highlight-bg-color: color-mix(in srgb, var(--red) 35%, transparent);
    --d2h-dark-ins-label-color: var(--green);
    --d2h-dark-del-label-color: var(--red);
    background: var(--bg);
  }
  .diff :global(.d2h-wrapper) {
    font-family: var(--mono);
  }
  .diff :global(.d2h-file-wrapper) {
    margin: 0;
    border: none;
    border-radius: 0;
  }
  .diff :global(.d2h-code-line),
  .diff :global(.d2h-code-side-line),
  .diff :global(.d2h-code-linenumber),
  .diff :global(.d2h-code-side-linenumber) {
    font-size: 12px;
  }
</style>
