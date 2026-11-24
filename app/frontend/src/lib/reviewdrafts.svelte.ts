// The diff notes not sent yet, per session: they outlive the Changes view
// (switching sessions or tabs keeps them) until sent or discarded. Not
// saved: quitting the app drops them.
import type { Note } from './review'

const drafts = $state<Record<string, Note[]>>({})
let nextId = 1

export function notesFor(sessionId: string): Note[] {
  return drafts[sessionId] ?? []
}

export function addNote(sessionId: string, n: Omit<Note, 'id'>) {
  drafts[sessionId] = [...notesFor(sessionId), { ...n, id: nextId++ }]
}

export function removeNote(sessionId: string, id: number) {
  drafts[sessionId] = notesFor(sessionId).filter((n) => n.id !== id)
}

export function clearNotes(sessionId: string) {
  delete drafts[sessionId]
}
