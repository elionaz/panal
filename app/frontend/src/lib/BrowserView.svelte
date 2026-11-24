<script lang="ts">
  // The Browser tab: a live image of the session's own Chrome (the one its
  // agent drives), its tabs, and the page's address. It only shows: the
  // agent keeps driving it.
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { api, errorMessage } from './api'
  import { t } from './i18n.svelte'

  interface Page {
    id: string
    title: string
    url: string
  }

  let { sessionId, onError }: { sessionId: string; onError: (msg: string) => void } = $props()

  let pages = $state<Page[]>([])
  let current = $state('')
  let image = $state('')
  /** The browser went away (its message, or '' when it just closed). */
  let ended = $state<string | null>(null)

  const page = $derived(pages.find((p) => p.id === current) ?? null)

  $effect(() => {
    const id = sessionId
    pages = []
    current = ''
    image = ''
    ended = null
    const offs = [
      EventsOn(`browser:pages:${id}`, (v: { pages: Page[] | null; current: string }) => {
        pages = v.pages ?? []
        current = v.current
        if (!v.current) image = ''
      }),
      // Frames only come from the page shown (a new one's first frame
      // can come before the pages say so).
      EventsOn(`browser:frame:${id}`, (f: { page: string; data: string }) => (image = `data:image/jpeg;base64,${f.data}`)),
      EventsOn(`browser:end:${id}`, (msg: string) => (ended = msg)),
    ]
    api.watchBrowser(id).catch((e) => (ended = errorMessage(e)))
    return () => {
      for (const off of offs) off()
      void api.unwatchBrowser(id).catch(() => {})
    }
  })

  function pick(p: Page) {
    if (p.id === current) return
    void api.pickBrowserPage(sessionId, p.id)
  }

  async function close() {
    try {
      await api.closeBrowser(sessionId)
    } catch (e) {
      onError(errorMessage(e))
    }
  }

  const label = (p: Page) => p.title || p.url || t('browser.blank')
</script>

<div class="browser">
  <div class="bar">
    <div class="tabs">
      {#each pages as p (p.id)}
        <button class="tab" class:on={p.id === current} title={p.url} onclick={() => pick(p)}>{label(p)}</button>
      {/each}
    </div>
    <span class="url" title={page?.url ?? ''}>{page?.url ?? ''}</span>
    <span class="ro" title={t('browser.viewOnlyHelp')}>{t('browser.viewOnly')}</span>
    <button class="close" title={t('browser.closeHelp')} onclick={close}>{t('browser.close')}</button>
  </div>
  <div class="screen">
    {#if ended !== null}
      <p class="empty">{ended ? t('browser.endedErr', { msg: ended }) : t('browser.ended')}</p>
    {:else if image}
      <img src={image} alt={page ? label(page) : ''} />
    {:else}
      <p class="empty">{pages.length || current ? t('browser.waiting') : t('browser.connecting')}</p>
    {/if}
  </div>
</div>

<style>
  .browser {
    position: absolute;
    inset: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    color: var(--fg);
    font-size: 13px;
  }
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 10px;
    border-bottom: 1px solid var(--border);
    min-width: 0;
  }
  .tabs {
    display: flex;
    gap: 4px;
    min-width: 0;
    flex: 0 1 auto;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tab {
    flex: none;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 3px 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: transparent;
    color: var(--fg-dim);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
  }
  .tab:hover {
    background: var(--hover);
  }
  .tab.on {
    background: var(--accent-bg);
    border-color: var(--border-strong);
    color: var(--fg);
  }
  .url {
    flex: 1 1 0;
    min-width: 60px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-family: var(--mono);
    font-size: 11.5px;
  }
  .ro {
    flex: none;
    color: var(--muted);
    font-size: 11.5px;
  }
  .close {
    flex: none;
    padding: 3px 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: transparent;
    color: var(--fg-dim);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
  }
  .close:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .screen {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 10px;
    background: var(--sidebar-bg);
  }
  img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: #fff;
    box-shadow: var(--shadow);
  }
  .empty {
    color: var(--muted);
    max-width: 420px;
    text-align: center;
    line-height: 1.5;
  }
</style>
