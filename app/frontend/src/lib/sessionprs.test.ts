import { describe, expect, it } from 'vitest'
import type { ForgeListItem } from './api'
import { groupPRs, selectedBranch, showPRList } from './sessionprs'

const item = (branch: string, current = false) => ({ branch, current, pr: { number: 1 } }) as unknown as ForgeListItem

describe('showPRList', () => {
  it('stays hidden for none, or only the current branch', () => {
    expect(showPRList(undefined)).toBe(false)
    expect(showPRList([])).toBe(false)
    expect(showPRList([item('feat/b', true)])).toBe(false)
  })
  it('shows several, or one of another branch', () => {
    expect(showPRList([item('feat/b', true), item('fix/a')])).toBe(true)
    expect(showPRList([item('fix/a')])).toBe(true)
  })
})

describe('groupPRs', () => {
  const pr = (branch: string, state: string, current = false) => ({ branch, current, pr: { number: 1, state } }) as unknown as ForgeListItem
  it('puts open and draft ones first, keeping the order', () => {
    const g = groupPRs([pr('a', 'merged'), pr('b', 'open', true), pr('c', 'closed'), pr('d', 'draft')])
    expect(g.open.map((x) => x.branch)).toEqual(['b', 'd'])
    expect(g.done.map((x) => x.branch)).toEqual(['a', 'c'])
  })
  it('handles no list', () => {
    expect(groupPRs(null)).toEqual({ open: [], done: [] })
  })
})

describe('selectedBranch', () => {
  const items = [item('feat/b', true), item('fix/a')]
  it('keeps a listed other branch', () => {
    expect(selectedBranch(items, 'fix/a')).toBe('fix/a')
  })
  it('falls back to the current branch (empty)', () => {
    expect(selectedBranch(items, '')).toBe('')
    expect(selectedBranch(items, 'feat/b')).toBe('')
    expect(selectedBranch(items, 'gone')).toBe('')
    expect(selectedBranch([], 'fix/a')).toBe('')
  })
})
