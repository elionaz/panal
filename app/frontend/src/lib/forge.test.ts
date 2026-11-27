import { describe, expect, it } from 'vitest'
import type { ForgeComment, ForgePR, ForgeStatus } from './api'
import { canMerge, commentPlace, forgeView, hostName, isMerged, mergeWarnings, prBody, prRef } from './forge'

const pr = (over: Partial<ForgePR> = {}): ForgePR =>
  ({
    number: 12, title: 't', state: 'open', url: 'u', base: 'main', head: 'feat/x',
    checks: { state: 'passing', total: 1, passed: 1, failed: 0, pending: 0 }, ...over,
  }) as ForgePR

describe('forgeView', () => {
  const st = (unavailable?: string) => ({ unavailable, ahead: 0 }) as ForgeStatus
  it('shows the PR block when the integration works', () => expect(forgeView(st())).toBe('pr'))
  it('hints at installing or logging in', () => {
    expect(forgeView(st('forge_cli_missing'))).toBe('install')
    expect(forgeView(st('forge_not_authed'))).toBe('login')
  })
  it('stays quiet on the base branch and other hosts', () => {
    for (const u of ['forge_on_base', 'forge_unsupported', 'pr_no_remote', 'not_a_repo']) expect(forgeView(st(u))).toBe('hidden')
    expect(forgeView(null)).toBe('hidden')
  })
})

describe('pull request helpers', () => {
  it('names hosts', () => {
    expect(hostName({ host: 'github.com', kind: 'github' } as ForgeStatus)).toBe('GitHub')
    expect(hostName({ host: 'gitlab.com', kind: 'gitlab' } as ForgeStatus)).toBe('GitLab')
    expect(hostName({ host: 'gitlab.example.org', kind: 'gitlab' } as ForgeStatus)).toBe('gitlab.example.org')
  })

  it('names PRs and MRs like their hosts', () => {
    expect(prRef('github', 12)).toBe('PR #12')
    expect(prRef('gitlab', 5)).toBe('MR !5')
  })

  it('merges only open ones', () => {
    expect(canMerge(pr())).toBe(true)
    expect(canMerge(pr({ state: 'draft' }))).toBe(true)
    expect(canMerge(pr({ state: 'merged' }))).toBe(false)
    expect(canMerge(null)).toBe(false)
    expect(isMerged(pr({ state: 'merged' }))).toBe(true)
    expect(isMerged(pr())).toBe(false)
    expect(isMerged(pr({ state: 'closed' }))).toBe(false)
    expect(isMerged(null)).toBe(false)
  })

  it('warns before a merge the host may refuse', () => {
    expect(mergeWarnings(pr())).toEqual([])
    const failing = { state: 'failing', total: 2, passed: 1, failed: 1, pending: 0 }
    expect(mergeWarnings(pr({ conflicts: true, blocked: true, checks: failing } as Partial<ForgePR>))).toEqual(['conflicts', 'checks'])
    expect(mergeWarnings(pr({ state: 'draft', blocked: true }))).toEqual(['draft', 'blocked'])
  })

  it('places line comments', () => {
    expect(commentPlace({ path: 'a.go', line: 42 } as ForgeComment)).toBe('a.go:42')
    expect(commentPlace({ path: 'a.go' } as ForgeComment)).toBe('a.go')
    expect(commentPlace({} as ForgeComment)).toBe('')
  })
})

describe('prBody', () => {
  it('appends the session section when kept', () => {
    expect(prBody('- a\n- b\n', '### Agent sessions\n', true)).toBe('- a\n- b\n\n### Agent sessions')
    expect(prBody('', '### Agent sessions', true)).toBe('### Agent sessions')
  })
  it('leaves the body alone otherwise', () => {
    expect(prBody('text', '### Agent sessions', false)).toBe('text')
    expect(prBody('text', undefined, true)).toBe('text')
    expect(prBody('text', '  ', true)).toBe('text')
  })
})
