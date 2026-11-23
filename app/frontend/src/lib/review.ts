// Feedback for a session's agent from Changes: notes on lines of the diff,
// and a pull request's failing checks or comments. Each becomes one panal
// message to the session (the agent reads it like any other).
import type { ForgeComment, ForgePR } from './api'
import { commentPlace, prRef } from './forge'
import type { MsgKey } from './locales/en'
import type { Params } from './i18n.svelte'

type Tr = (key: MsgKey, params?: Params) => string

/** Which version of the file a line number counts in: the new one (added
 * or unchanged lines) or the old one (removed lines). */
export type Side = 'new' | 'old'

/** A clicked line of the diff. */
export interface LineRef {
  path: string
  side: Side
  line: number
  code: string
}

/** A note on one line or a run of lines of one file. */
export interface Note {
  id: number
  path: string
  side: Side
  start: number
  end: number
  /** The lines' text when the note was written (lines move as the agent
   * edits: the text says which ones were meant). */
  code: string[]
  text: string
}

/** The line a diff number cell stands for: in the unified view a row has
 * the old and the new number (the new one wins: removed lines only have the
 * old one); side by side, the column says which. */
export function pickLine(oldNum: string, newNum: string): { side: Side; line: number } | null {
  const n = parseInt(newNum.trim(), 10)
  if (n > 0) return { side: 'new', line: n }
  const o = parseInt(oldNum.trim(), 10)
  if (o > 0) return { side: 'old', line: o }
  return null
}

/** The selection from anchor to the clicked line: a range when both are in
 * the same file and version, else the clicked line alone. */
export function extend(anchor: LineRef | null, clicked: LineRef, shift: boolean, codeOf: (l: number) => string): Omit<Note, 'id' | 'text'> {
  if (shift && anchor && anchor.path === clicked.path && anchor.side === clicked.side) {
    const start = Math.min(anchor.line, clicked.line)
    const end = Math.max(anchor.line, clicked.line)
    const code: string[] = []
    for (let l = start; l <= end && code.length < MAX_LINES; l++) code.push(codeOf(l))
    return { path: clicked.path, side: clicked.side, start, end, code }
  }
  return {
    path: clicked.path,
    side: clicked.side,
    start: clicked.line,
    end: clicked.line,
    code: [clicked.code],
  }
}

/** Lines of code quoted per note: the agent opens the file for more. */
export const MAX_LINES = 8

/** "a.go:42", "a.go:42-44", or the removed lines of the old version. */
export function notePlace(n: Pick<Note, 'path' | 'side' | 'start' | 'end'>, tr: Tr): string {
  const lines = n.start === n.end ? `${n.start}` : `${n.start}-${n.end}`
  return n.side === 'new' ? `${n.path}:${lines}` : tr('review.placeOld', { path: n.path, lines })
}

/** Whether a note covers a line. */
export function covers(n: Note, path: string, side: Side, line: number): boolean {
  return n.path === path && n.side === side && line >= n.start && line <= n.end
}

function quote(code: string[]): string {
  return code.map((l) => `> ${l.replace(/\s+$/, '')}`).join('\n')
}

/** The message with every note, in file and line order. */
export function reviewMessage(notes: Note[], tr: Tr): string {
  const sorted = [...notes].sort((a, b) => a.path.localeCompare(b.path) || a.start - b.start)
  const items = sorted.map((n, i) => {
    const parts = [`${i + 1}. ${notePlace(n, tr)}`]
    if (n.code.some((l) => l.trim())) parts.push(quote(n.code))
    parts.push(n.text.trim())
    return parts.join('\n')
  })
  return `${tr('review.msgHead', { n: notes.length })}\n\n${items.join('\n\n')}`
}

/** How to look at the failures with the host's CLI. */
function checksCmd(kind: string | undefined, pr: ForgePR): string {
  return kind === 'gitlab' ? `glab ci status --branch ${pr.head}` : `gh pr checks ${pr.number}`
}

/** Asks the agent to fix a pull request's failing checks. */
export function checksMessage(kind: string | undefined, pr: ForgePR, tr: Tr): string {
  const names = pr.checks.failing?.length ? pr.checks.failing.join(', ') : ''
  return tr(names ? 'review.checksMsg' : 'review.checksMsgNoNames', {
    ref: prRef(kind, pr.number),
    branch: pr.head,
    names,
    cmd: checksCmd(kind, pr),
    n: pr.checks.failed,
  })
}

/** Asks the agent to address a pull request's comments (one or all). */
export function commentsMessage(kind: string | undefined, pr: ForgePR, comments: ForgeComment[], tr: Tr, has: (key: string) => boolean = () => true): string {
  const items = comments.map((c) => {
    const place = commentPlace(c)
    const head = place ? tr('review.commentOn', { author: c.author, place }) : c.author
    const rk = `pr.review.label.${c.review ?? ''}`
    const review = c.review ? (has(rk) ? tr(rk as MsgKey) : c.review) : ''
    const parts = [`— ${head}${review ? ` (${review})` : ''}:`]
    if (c.body.trim()) parts.push(c.body.trim())
    if (c.url) parts.push(c.url)
    return parts.join('\n')
  })
  return `${tr('review.commentsMsg', { ref: prRef(kind, pr.number), branch: pr.head, n: comments.length })}\n\n${items.join('\n\n')}`
}
