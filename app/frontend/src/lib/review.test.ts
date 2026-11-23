import { describe, expect, it } from 'vitest'
import type { ForgeComment, ForgePR } from './api'
import { checksMessage, commentsMessage, covers, extend, notePlace, pickLine, reviewMessage, type Note } from './review'

// A translator that shows the key and its parameters.
const tr = (k: string, p?: Record<string, unknown>) => (p ? `${k}${JSON.stringify(p)}` : k)

const note = (over: Partial<Note>): Note => ({
  id: 1,
  path: 'a.go',
  side: 'new',
  start: 3,
  end: 3,
  code: ['x := 1'],
  text: 'rename x',
  ...over,
})

describe('pickLine', () => {
  it('prefers the new number, falls back to the old one', () => {
    expect(pickLine('4', '5')).toEqual({ side: 'new', line: 5 })
    expect(pickLine('4', '')).toEqual({ side: 'old', line: 4 })
    expect(pickLine(' ', ' ')).toBeNull()
  })
})

describe('extend', () => {
  const code = (l: number) => `line ${l}`
  const at = (line: number, path = 'a.go', side: 'new' | 'old' = 'new') => ({
    path,
    side,
    line,
    code: code(line),
  })
  it('a click alone is one line', () => {
    expect(extend(at(2), at(7), false, code)).toMatchObject({
      start: 7,
      end: 7,
      code: ['line 7'],
    })
  })
  it('shift-click makes a range either way', () => {
    expect(extend(at(7), at(5), true, code)).toMatchObject({
      start: 5,
      end: 7,
      code: ['line 5', 'line 6', 'line 7'],
    })
  })
  it('no range across files or versions', () => {
    expect(extend(at(2, 'b.go'), at(5), true, code)).toMatchObject({
      start: 5,
      end: 5,
    })
    expect(extend(at(2, 'a.go', 'old'), at(5), true, code)).toMatchObject({
      start: 5,
      end: 5,
    })
  })
  it('quotes a long range only in part', () => {
    expect(extend(at(1), at(40), true, code).code).toHaveLength(8)
  })
})

describe('notes', () => {
  it('places them', () => {
    expect(notePlace(note({}), tr)).toBe('a.go:3')
    expect(notePlace(note({ end: 5 }), tr)).toBe('a.go:3-5')
    expect(notePlace(note({ side: 'old' }), tr)).toBe('review.placeOld{"path":"a.go","lines":"3"}')
  })
  it('covers their lines only', () => {
    const n = note({ end: 5 })
    expect(covers(n, 'a.go', 'new', 4)).toBe(true)
    expect(covers(n, 'a.go', 'new', 6)).toBe(false)
    expect(covers(n, 'a.go', 'old', 4)).toBe(false)
  })
  it('one message, in file and line order, with the code quoted', () => {
    const m = reviewMessage([note({ path: 'b.go', start: 1, end: 1, text: 'second' }), note({ text: ' first ', code: ['x := 1  '] })], tr)
    expect(m).toBe('review.msgHead{"n":2}\n\n1. a.go:3\n> x := 1\nfirst\n\n2. b.go:1\n> x := 1\nsecond')
  })
  it('no quote for blank lines', () => {
    expect(reviewMessage([note({ code: [''] })], tr)).toBe('review.msgHead{"n":1}\n\n1. a.go:3\nrename x')
  })
})

const pr = (failing?: string[]): ForgePR =>
  ({
    number: 12,
    title: 'T',
    state: 'open',
    url: 'u',
    base: 'main',
    head: 'feat/x',
    checks: {
      state: 'failing',
      total: 3,
      passed: 1,
      failed: 2,
      pending: 0,
      failing,
    },
  }) as ForgePR

describe('pull request messages', () => {
  it('failing checks name the checks and how to look', () => {
    expect(checksMessage('github', pr(['lint', 'test']), tr)).toBe(
      'review.checksMsg{"ref":"PR #12","branch":"feat/x","names":"lint, test","cmd":"gh pr checks 12","n":2}',
    )
    expect(checksMessage('gitlab', pr(), tr)).toContain(
      'review.checksMsgNoNames{"ref":"MR !12","branch":"feat/x","names":"","cmd":"glab ci status --branch feat/x"',
    )
  })
  it('comments with author, place, review and link', () => {
    const cs = [
      {
        kind: 'review',
        author: 'ana',
        body: 'Use a map.',
        path: 'a.go',
        line: 9,
        review: 'changes_requested',
        url: 'https://x/1',
      },
      { kind: 'comment', author: 'bo', body: ' ok ', review: 'weird' },
    ] as ForgeComment[]
    expect(commentsMessage('github', pr(), cs, tr, (k) => k.endsWith('changes_requested'))).toBe(
      'review.commentsMsg{"ref":"PR #12","branch":"feat/x","n":2}\n\n' +
        '— review.commentOn{"author":"ana","place":"a.go:9"} (pr.review.label.changes_requested):\nUse a map.\nhttps://x/1\n\n' +
        '— bo (weird):\nok',
    )
  })
})
