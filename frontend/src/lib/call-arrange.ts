import { gridFor, layoutFor } from './call-model'
import type { ParticipantView } from './call-room'

/** Strip beside a large tile: this many tiles, then "N người khác". */
export const STRIP_TILES = 4

export type Arrangement =
  /** Nobody yet (the room is still opening). */
  | { kind: 'empty' }
  /** Alone in the room. */
  | { kind: 'single'; self: ParticipantView }
  /** Two people: the other large, you in the corner. */
  | { kind: 'pip'; main: ParticipantView; self: ParticipantView }
  | { kind: 'grid'; cols: 2 | 3 | 4; perPage: number; tiles: ParticipantView[] }
  /** One large picture (a presentation, a pinned or speaking person) and a strip of the others. */
  | { kind: 'focus'; main: ParticipantView; show: 'camera' | 'screen'; strip: ParticipantView[]; more: number }

export interface ArrangeInput {
  participants: ParticipantView[]
  presenterId: string | null
  speakerId: string | null
  pinnedId: string | null
  mode: 'auto' | 'speaker' | 'grid'
}

function focus(main: ParticipantView, show: 'camera' | 'screen', rest: ParticipantView[]): Arrangement {
  // Whoever is speaking goes first in the strip, so the strip is never all silent faces.
  const ordered = [...rest].sort((a, b) => Number(b.speaking) - Number(a.speaking))
  return { kind: 'focus', main, show, strip: ordered.slice(0, STRIP_TILES), more: Math.max(0, ordered.length - STRIP_TILES) }
}

/** Decides what the stage shows for the people in the room (mockup `video-call.html` §4 and §5). */
export function arrange({ participants, presenterId, speakerId, pinnedId, mode }: ArrangeInput): Arrangement {
  const self = participants.find((p) => p.isLocal) ?? participants[0]
  if (!self) return { kind: 'empty' }
  const others = participants.filter((p) => !p.isLocal)

  const presenter = presenterId ? participants.find((p) => p.id === presenterId) : undefined
  if (presenter?.screen) return focus(presenter, 'screen', participants)

  if (participants.length <= 1) return { kind: 'single', self }

  const pinned = pinnedId ? participants.find((p) => p.id === pinnedId) : undefined
  if (pinned) return focus(pinned, 'camera', participants.filter((p) => p.id !== pinned.id))

  if (mode === 'speaker') {
    const main = others.find((p) => p.id === speakerId) ?? others[0] ?? self
    return focus(main, 'camera', participants.filter((p) => p.id !== main.id))
  }
  if (mode === 'grid') {
    const g = gridFor(participants.length)
    return { kind: 'grid', cols: g.cols, perPage: g.perPage, tiles: participants }
  }

  const l = layoutFor(participants.length)
  if (l.kind === 'pip' && others[0]) return { kind: 'pip', main: others[0], self }
  if (l.kind === 'grid') return { kind: 'grid', cols: l.cols, perPage: l.perPage, tiles: participants }
  return { kind: 'single', self }
}
