<script lang="ts">
  // The branch's pull request on GitHub / GitLab, in Changes: state, checks
  // and review, its comments, merging it on the host, or opening it. Runs
  // through the user's own gh / glab login (internal/forge); without one it
  // shows how to set it up, and on the base branch it shows nothing.
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
  import { api, errorMessage, type ForgeComment, type ForgeStatus } from './api'
  import { canMerge, commentPlace, forgeView, hostName, isMerged, mergeWarnings, prBody, prRef } from './forge'
  import { hasKey, locale, t, type Params } from './i18n.svelte'
  import { checksMessage, commentsMessage } from './review'

  interface Props {
    sessionId?: string
    worktreeId?: string
    /** Changes with the daemon's git info (pushes, commits): reload. */
    reloadKey?: string
    onError: (msg: string) => void
    /** A line comment's file was clicked (select it in the diff). */
    onSelectFile?: (path: string) => void
    /** The pull request was merged on the host. */
    onMerged?: (msg: string) => void
    /** The integration works here (Changes hides its push-and-browse button). */
    available?: boolean
    /** The branch has a pull request (Changes then hides its create button). */
    hasPR?: boolean
    /** The head branch of the shown pull request when it is merged (here or
     * on the host); '' otherwise. */
    mergedHead?: string
    /** The create form is open (Changes shows its button as pressed). */
    creating?: boolean
    /** Another branch the session worked on ('' = the one it is on now):
     * its pull request is shown, and none is created for it. */
    branch?: string
    /** The session's name when it can take messages: the checks and
     * comments can then be sent to its agent. */
    sendTo?: string
    /** Sends a message to the session; false when it failed (reported). */
    onSend?: (body: string) => Promise<boolean>
  }

  let {
    sessionId,
    worktreeId,
    reloadKey = '',
    onError,
    onSelectFile,
    onMerged,
    available = $bindable(false),
    hasPR = $bindable(false),
    mergedHead = $bindable(''),
    creating = $bindable(false),
    branch = '',
    sendTo = '',
    onSend,
  }: Props = $props()

  let status = $state<ForgeStatus | null>(null)
  let comments = $state<ForgeComment[] | null>(null)
  let commentsOpen = $state(false)
  let commentsLoading = $state(false)
  let merging = $state(false)
  let method = $state('')
  let deleteBranch = $state(true)
  let title = $state('')
  let body = $state('')
  let draft = $state(false)
  let withSession = $state(true)
  let busy = $state(false)
  let notice = $state('')

  const view = $derived(forgeView(status))
  const pr = $derived(status?.pr ?? null)
  const host = $derived(hostName(status))
  const ref = $derived(pr ? prRef(status?.kind, pr.number) : '')

  $effect(() => {
    available = view === 'pr'
  })
  $effect(() => {
    hasPR = !!pr
  })
  $effect(() => {
    mergedHead = pr && isMerged(pr) ? pr.head : ''
  })
  /** Without a pull request there is nothing to show until the create form
   * opens (Changes has the button) or there is a notice. */
  const quiet = $derived(!pr && !creating && !notice && !status?.warning)

  /** The target as a key: results that come back for another one are
   * dropped (the user switched session while gh was running). */
  const target = () => `${sessionId ?? ''}|${worktreeId ?? ''}|${branch}`

  // One status request in flight per block: reloads asked for meanwhile
  // run once it is back.
  let seq = 0
  let inFlight = false
  let again = false
  async function load() {
    if (inFlight) {
      again = true
      return
    }
    inFlight = true
    const my = ++seq
    const tgt = target()
    try {
      const s = await api.forgeStatus(sessionId ?? '', worktreeId ?? '', branch)
      if (my !== seq || tgt !== target()) return
      status = s
      if (!merging) method = s.mergeMethods?.[0] ?? ''
    } catch (e) {
      if (my !== seq || tgt !== target()) return
      status = null
      // A failing host (network, gh error) is reported once, not on every
      // reload.
      const msg = errorMessage(e)
      if (`${tgt}|${msg}` !== lastError) {
        lastError = `${tgt}|${msg}`
        onError(msg)
      }
    } finally {
      inFlight = false
      if (again) {
        again = false
        void load()
      }
    }
  }
  let lastError = ''

  let lastKey = ''
  $effect(() => {
    const key = `${target()}|${reloadKey}`
    if (key === lastKey) return
    const targetChanged = lastKey.split('|').slice(0, 2).join('|') !== key.split('|').slice(0, 2).join('|')
    lastKey = key
    if (targetChanged) {
      status = null
      comments = null
      commentsOpen = merging = creating = false
      notice = ''
      seq++ // an answer for the previous target is not this one's
      inFlight = again = false
    } else if (commentsOpen) {
      void loadComments()
    }
    void load()
  })

  let cseq = 0
  async function loadComments() {
    if (!pr) return
    const my = ++cseq
    commentsLoading = true
    try {
      const cs = await api.forgeComments(sessionId ?? '', worktreeId ?? '', pr.number, branch)
      if (my === cseq) comments = cs
    } catch (e) {
      if (my === cseq) onError(errorMessage(e))
    } finally {
      if (my === cseq) commentsLoading = false
    }
  }

  function toggleComments() {
    commentsOpen = !commentsOpen
    if (commentsOpen) void loadComments()
  }

  function openMerge() {
    creating = false
    merging = !merging
    method = status?.mergeMethods?.[0] ?? ''
  }

  async function doMerge() {
    if (!pr || !method) return
    busy = true
    const r = ref
    const tgt = target()
    try {
      const s = await api.forgeMerge(sessionId ?? '', worktreeId ?? '', pr.number, method, pr.head, deleteBranch, branch)
      if (tgt !== target()) return
      seq++ // a status read started before the merge is older than this
      status = s
      merging = false
      if (s.result === 'queued') {
        notice = t('pr.queued', { ref: r, host })
      } else {
        const msg = t('pr.merged', { ref: r, host })
        if (onMerged) onMerged(msg)
        else notice = msg
      }
    } catch (e) {
      if (tgt === target()) onError(errorMessage(e))
    } finally {
      busy = false
    }
  }

  /** Opens (or closes) the create form: Changes' Create pull request. */
  export function openCreate() {
    merging = false
    creating = !creating
    if (creating) {
      title = status?.title ?? ''
      body = status?.body ?? ''
      draft = false
      withSession = true
    }
  }

  async function doCreate() {
    busy = true
    const tgt = target()
    try {
      const s = await api.forgeCreate(sessionId ?? '', worktreeId ?? '', title, prBody(body, status?.session, withSession), draft)
      if (tgt !== target()) return
      seq++
      status = s
      creating = false
      if (s.pr) notice = t('pr.created', { ref: prRef(s.kind, s.pr.number), host })
    } catch (e) {
      if (tgt === target()) onError(errorMessage(e))
    } finally {
      busy = false
    }
  }

  /** A message whose key ends in a value from the host (state, method…);
   * an unknown value shows as it is. */
  function tv(key: string, params?: Params): string {
    return hasKey(key) ? t(key, params) : key.slice(key.lastIndexOf('.') + 1)
  }

  const canSend = $derived(!!sendTo && !!onSend)
  let sending = $state(false)
  async function send(body: string) {
    if (!onSend || sending) return
    sending = true
    try {
      if (await onSend(body)) {
        const msg = t('pr.sent', { name: sendTo })
        notice = msg
        window.setTimeout(() => notice === msg && (notice = ''), 4000)
      }
    } finally {
      sending = false
    }
  }

  function when(at: string | undefined): string {
    if (!at) return ''
    const d = new Date(at)
    return isNaN(d.getTime()) ? '' : d.toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' })
  }
</script>

{#if view === 'install' || view === 'login'}
  {@const d = (status?.unavailableData ?? {}) as Record<string, string>}
  <div class="pr hint">
    {view === 'install'
      ? t('pr.hintInstall', { tool: d.tool ?? 'gh', install: d.install ?? 'brew install gh' })
      : t('pr.hintLogin', { tool: d.tool ?? 'gh' })}
  </div>
{:else if view === 'pr' && status && !quiet}
  <div class="pr">
    {#if notice}<p class="ok">{notice}</p>{/if}
    {#if status.warning}<p class="warn">{status.warning}</p>{/if}
    {#if pr}
      <div class="line">
        <button class="ref" title={t('pr.openOn', { host })} onclick={() => BrowserOpenURL(pr.url)}>{ref}</button>
        <span class="state st-{pr.state}">{tv(`pr.state.${pr.state}`)}</span>
        <span class="title" title={pr.title}>{pr.title}</span>
        {#if pr.checks.state !== 'none'}
          <span class="checks ck-{pr.checks.state}" title={pr.checks.failing?.join(', ') ?? ''}>
            {pr.checks.state === 'passing' ? '✓' : pr.checks.state === 'failing' ? '✗' : '●'}
            {pr.checks.state === 'failing' ? t('pr.checks.failing', { n: pr.checks.failed }) : tv(`pr.checks.${pr.checks.state}`)}
          </span>
        {/if}
        {#if pr.review}<span class="review rv-{pr.review}">{tv(`pr.review.${pr.review}`)}</span>{/if}
        {#if canSend && pr.checks.state === 'failing' && canMerge(pr)}
          <button class="btn" disabled={sending} onclick={() => pr && send(checksMessage(status?.kind, pr, t))}>{t('pr.sendChecks', { name: sendTo })}</button>
        {/if}
        <span class="spacer"></span>
        <button class="btn" onclick={toggleComments}>
          {commentsOpen ? t('pr.hideComments') : comments ? t('pr.comments', { n: comments.length }) : t('pr.showComments')}
        </button>
        {#if canMerge(pr) && status.mergeMethods?.length}
          <button class="btn" disabled={busy} onclick={openMerge}>{t('pr.merge', { host })}</button>
        {/if}
        <button class="btn" onclick={() => BrowserOpenURL(pr.url)}>{t('pr.openOn', { host })}</button>
      </div>
    {/if}

    {#if merging && pr}
      <div class="form">
        <p>{t('pr.mergeInfo', { ref, base: pr.base, host })}</p>
        {#each mergeWarnings(pr) as w (w)}
          <p class="warn">{tv(`pr.warn.${w}`, { host, base: pr.base })}</p>
        {/each}
        <div class="row">
          {#if (status.mergeMethods?.length ?? 0) > 1}
            <select bind:value={method}>
              {#each status.mergeMethods ?? [] as m (m)}<option value={m}>{tv(`pr.method.${m}`)}</option>{/each}
            </select>
          {:else}
            <span class="muted">{tv(`pr.method.${method}`)}</span>
          {/if}
          <label class="check"><input type="checkbox" bind:checked={deleteBranch} /> {t('pr.deleteBranch', { branch: pr.head, host })}</label>
        </div>
        <div class="row">
          <button class="btn primary" disabled={busy || !method || pr.state === 'draft'} onclick={doMerge}>{t('pr.doMerge')}</button>
          <button class="btn" onclick={() => (merging = false)}>{t('pr.cancel')}</button>
        </div>
      </div>
    {/if}

    {#if creating && !pr}
      <div class="form">
        <p>{t('pr.createInfo', { branch: status.branch ?? '', base: status.base ?? '', host })}</p>
        <label class="field">{t('pr.titleLabel')}<input type="text" bind:value={title} /></label>
        <label class="field">{t('pr.bodyLabel')}<textarea rows="5" bind:value={body}></textarea></label>
        {#if status.session}
          <label class="check"><input type="checkbox" bind:checked={withSession} /> {t('pr.withSession')}</label>
          {#if withSession}
            <details class="session">
              <summary>{t('pr.sessionPreview')}</summary>
              <pre>{status.session}</pre>
            </details>
          {/if}
        {/if}
        <label class="check"><input type="checkbox" bind:checked={draft} /> {t('pr.draft')}</label>
        <div class="row">
          <button class="btn primary" disabled={busy || !status.ahead || !title.trim()} onclick={doCreate}>
            {t('pr.doCreate', { n: status.ahead })}
          </button>
          <button class="btn" onclick={() => (creating = false)}>{t('pr.cancel')}</button>
        </div>
      </div>
    {/if}

    {#if commentsOpen && pr}
      <div class="comments">
        {#if commentsLoading && !comments}
          <p class="muted">{t('pr.loadingComments')}</p>
        {:else if comments && !comments.length}
          <p class="muted">{t('pr.noComments')}</p>
        {:else if comments}
          {#if canSend}
            <div class="sendall">
              <button class="btn" disabled={sending} onclick={() => pr && comments && send(commentsMessage(status?.kind, pr, comments, t, hasKey))}
                >{t('pr.sendComments', { name: sendTo })}</button
              >
            </div>
          {/if}
          {#each comments as c, i (i)}
            <div class="comment">
              <div class="meta">
                <b>{c.author}</b>
                {#if c.review}<span class="review rv-{c.review}">{tv(`pr.review.label.${c.review}`)}</span>{/if}
                {#if commentPlace(c)}
                  <button class="place" onclick={() => onSelectFile?.(c.path ?? '')}>{commentPlace(c)}</button>
                {/if}
                <span class="muted">{when(c.at)}</span>
                <span class="spacer"></span>
                {#if canSend}<button class="link" disabled={sending} onclick={() => pr && send(commentsMessage(status?.kind, pr, [c], t, hasKey))}
                    >{t('pr.sendComment', { name: sendTo })}</button
                  >{/if}
                {#if c.url}<button class="link" onclick={() => BrowserOpenURL(c.url ?? '')}>{t('pr.openOn', { host })}</button>{/if}
              </div>
              {#if c.body.trim()}<div class="body">{c.body}</div>{/if}
            </div>
          {/each}
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .pr {
    flex: none;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    font-size: 12.5px;
    color: var(--fg-dim);
  }
  .pr.hint {
    color: var(--muted);
    font-size: 12px;
  }
  .line,
  .row,
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .row {
    margin-top: 8px;
    flex-wrap: wrap;
  }
  .spacer {
    flex: 1;
  }
  .ref {
    font-weight: 600;
    color: var(--blue);
  }
  .ref:hover,
  .place:hover,
  .link:hover {
    text-decoration: underline;
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--fg);
  }
  .state,
  .review {
    flex: none;
    padding: 1px 7px;
    border-radius: 10px;
    font-size: 11px;
    background: var(--hover);
  }
  .st-open {
    color: var(--green);
    background: color-mix(in srgb, var(--green) 14%, transparent);
  }
  .st-merged {
    color: var(--blue);
    background: color-mix(in srgb, var(--blue) 14%, transparent);
  }
  .st-closed {
    color: var(--red);
    background: color-mix(in srgb, var(--red) 12%, transparent);
  }
  .checks {
    flex: none;
    font-size: 12px;
  }
  .ck-passing,
  .rv-approved {
    color: var(--green);
  }
  .ck-failing,
  .rv-changes_requested {
    color: var(--red);
  }
  .ck-pending,
  .rv-review_required {
    color: var(--amber);
  }
  .btn {
    flex: none;
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
  .form {
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px dashed var(--border);
  }
  .form p {
    margin: 0 0 6px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 6px 0;
    font-size: 11.5px;
    color: var(--muted);
  }
  .field input,
  .field textarea,
  select {
    font: inherit;
    font-size: 12.5px;
    color: var(--fg);
    background: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    padding: 4px 7px;
  }
  .field textarea {
    resize: vertical;
    font-family: inherit;
  }
  .session {
    margin: 2px 0 6px 22px;
    font-size: 11.5px;
    color: var(--muted);
  }
  .session summary {
    cursor: pointer;
  }
  .session pre {
    max-height: 180px;
    overflow: auto;
    margin: 4px 0 0;
    padding: 6px 8px;
    font-size: 11px;
    white-space: pre-wrap;
    word-break: break-word;
    color: var(--fg);
    background: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: 6px;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .muted {
    color: var(--muted);
  }
  .warn {
    color: var(--amber);
  }
  .ok {
    margin: 0 0 6px;
    color: var(--green);
  }
  .comments {
    margin-top: 8px;
    max-height: 280px;
    overflow-y: auto;
    border-top: 1px dashed var(--border);
  }
  .sendall {
    display: flex;
    justify-content: flex-end;
    padding: 6px 0 2px;
  }
  .comment {
    padding: 7px 0;
    border-bottom: 1px solid var(--border);
  }
  .comment:last-child {
    border-bottom: none;
  }
  .meta b {
    color: var(--fg);
    font-weight: 600;
  }
  .place {
    font-family: var(--mono, ui-monospace, Menlo, monospace);
    font-size: 11.5px;
    color: var(--blue);
  }
  .link {
    font-size: 11.5px;
    color: var(--muted);
  }
  .body {
    margin-top: 4px;
    white-space: pre-wrap;
    word-break: break-word;
    color: var(--fg-dim);
  }
</style>
