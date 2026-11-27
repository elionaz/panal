// Pull requests on the code host (forge.status): what the PR block in
// Changes shows. See internal/forge.
import type { ForgeComment, ForgePR, ForgeStatus } from './api'

/** What the block shows: nothing, a hint to set up gh / glab, or the PR
 * (or the offer to create one). */
export type ForgeView = 'hidden' | 'install' | 'login' | 'pr'

export function forgeView(s: ForgeStatus | null): ForgeView {
  if (!s) return 'hidden'
  switch (s.unavailable ?? '') {
    case '':
      return 'pr'
    case 'forge_cli_missing':
      return 'install'
    case 'forge_not_authed':
      return 'login'
  }
  // On the base branch, no remote, not GitHub / GitLab: nothing to say.
  return 'hidden'
}

/** The host as people call it: GitHub, GitLab, or a self-hosted name. */
export function hostName(s: Pick<ForgeStatus, 'host' | 'kind'> | null): string {
  const h = s?.host ?? ''
  if (h === 'github.com' || (!h && s?.kind === 'github')) return 'GitHub'
  if (h === 'gitlab.com' || (!h && s?.kind === 'gitlab')) return 'GitLab'
  return h
}

/** "PR #12" on GitHub, "MR !5" on GitLab. */
export function prRef(kind: string | undefined, n: number): string {
  return kind === 'gitlab' ? `MR !${n}` : `PR #${n}`
}

/** Open (or draft) pull requests can be merged from the app; GitHub does
 * not merge drafts. */
export function canMerge(pr: ForgePR | null | undefined): boolean {
  return !!pr && (pr.state === 'open' || pr.state === 'draft')
}

/** The pull request was merged, in the app or on the host. */
export function isMerged(pr: ForgePR | null | undefined): boolean {
  return pr?.state === 'merged'
}

/** Reasons the host may refuse or the user may want to wait. */
export function mergeWarnings(pr: ForgePR): Array<'conflicts' | 'checks' | 'pending' | 'blocked' | 'draft'> {
  const w: Array<'conflicts' | 'checks' | 'pending' | 'blocked' | 'draft'> = []
  if (pr.state === 'draft') w.push('draft')
  if (pr.conflicts) w.push('conflicts')
  if (pr.checks.state === 'failing') w.push('checks')
  else if (pr.checks.state === 'pending') w.push('pending')
  if (pr.blocked && !pr.conflicts) w.push('blocked')
  return w
}

/** "a.go:42" for line comments, '' otherwise. */
export function commentPlace(c: ForgeComment): string {
  if (!c.path) return ''
  return c.line ? `${c.path}:${c.line}` : c.path
}

/** The body a new pull request is opened with: what the user wrote, plus
 * the daemon's "Agent sessions" section when they keep it. */
export function prBody(body: string, session: string | undefined, include: boolean): string {
  const extra = include ? (session ?? '').trim() : ''
  const own = body.trimEnd()
  if (!extra) return own
  return own ? `${own}\n\n${extra}` : extra
}
