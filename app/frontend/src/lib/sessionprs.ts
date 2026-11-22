// The pull requests of a session's branches (forge.list), as Changes
// shows them.
import type { ForgeListItem } from './api'

/** Whether Changes lists them: more than one, or one that is not the
 * branch the folder is on (the PR block alone shows that case). */
export function showPRList(items: ForgeListItem[] | null | undefined): boolean {
  if (!items?.length) return false
  return items.length > 1 || !items[0].current
}

/** The list split for Changes: the open (and draft) ones, shown first,
 * and the merged or closed ones, folded away. Order is kept. */
export function groupPRs(items: ForgeListItem[] | null | undefined): { open: ForgeListItem[]; done: ForgeListItem[] } {
  const open: ForgeListItem[] = []
  const done: ForgeListItem[] = []
  for (const it of items ?? []) {
    const st = it.pr?.state
    ;(st === 'open' || st === 'draft' ? open : done).push(it)
  }
  return { open, done }
}

/** The list item the PR block shows: the selected branch while it is
 * listed, else the current branch's, else none (the block's default). */
export function selectedBranch(items: ForgeListItem[] | null | undefined, picked: string): string {
  if (!items?.length) return ''
  if (picked && items.some((it) => it.branch === picked && !it.current)) return picked
  return ''
}
