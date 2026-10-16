/**
 * What a call screen decides without touching a browser: how many columns the
 * grid has, which page a tile is on, how long a call has run, what a data
 * message may say. Everything here is plain data so it is tested directly.
 */

import type { CallSnapshot } from './call-room'

/** Data-message topics inside a room. Both stay inside the session; nothing is stored. */
export const HAND_TOPIC = 'nexus.hand'
export const REACTION_TOPIC = 'nexus.reaction'
/** Sent when someone starts presenting, so a presenter already on air steps down. */
export const SHARE_TOPIC = 'nexus.share'

/** How long an incoming call rings before it counts as missed: the server's 45 s, shown and sounded by the client. */
export const RING_MS = 45_000

/** How long a reaction floats, and how long after speaking a raised hand comes down on its own (DESIGN.md §6). */
export const REACTION_MS = 3000
export const HAND_AUTO_LOWER_MS = 2000

/** The six reactions, in the order of the popover. */
export const REACTIONS = [
  { emoji: '👍', label: 'Thích' },
  { emoji: '❤️', label: 'Thương' },
  { emoji: '😂', label: 'Cười' },
  { emoji: '😮', label: 'Ngạc nhiên' },
  { emoji: '👏', label: 'Vỗ tay' },
  { emoji: '🎉', label: 'Chúc mừng' },
] as const

const REACTION_SET = new Set<string>(REACTIONS.map((r) => r.emoji))

export type TileLayout =
  /** Alone in the room: your own tile, large. */
  | { kind: 'single'; cols: 1; perPage: 1 }
  /** One other person large, you small in the corner. */
  | { kind: 'pip'; cols: 1; perPage: 1 }
  | { kind: 'grid'; cols: 2 | 3 | 4; perPage: number }

/** Rows the grid shows before it pages. */
const ROWS = 3

/**
 * The automatic layout for `tiles` people on stage, you included
 * (mockup `video-call.html` §4): 2 → the other person large; 3–4 → 2 columns;
 * 5–9 → 3; 10–20 → 4, paged when the rows do not fit.
 */
export function layoutFor(tiles: number): TileLayout {
  if (tiles <= 1) return { kind: 'single', cols: 1, perPage: 1 }
  if (tiles === 2) return { kind: 'pip', cols: 1, perPage: 1 }
  return gridFor(tiles)
}

/** The grid for `tiles` people, used for 2 as well when the reader picks "Lưới". */
export function gridFor(tiles: number): TileLayout & { kind: 'grid' } {
  const cols = tiles <= 4 ? 2 : tiles <= 9 ? 3 : 4
  return { kind: 'grid', cols, perPage: cols * ROWS }
}

export function pageCount(total: number, perPage: number): number {
  return Math.max(1, Math.ceil(total / Math.max(1, perPage)))
}

export function pageOf<T>(items: readonly T[], page: number, perPage: number): T[] {
  const last = pageCount(items.length, perPage) - 1
  const at = Math.min(Math.max(0, page), last)
  return items.slice(at * perPage, (at + 1) * perPage)
}

const pad2 = (n: number) => String(n).padStart(2, '0')

/** `04:12`, or `1:02:05` past the hour. */
export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return h > 0 ? `${h}:${pad2(m)}:${pad2(s)}` : `${pad2(m)}:${pad2(s)}`
}

export type SignalQuality = 'good' | 'poor' | 'unknown'

export interface HandMessage { raised: boolean; at: number }
export interface ReactionMessage { emoji: string }
export interface ShareMessage { action: 'start' }

const decoder = typeof TextDecoder !== 'undefined' ? new TextDecoder() : null
const encoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null

/** Data messages are tiny; anything bigger is not ours. */
const MAX_PAYLOAD = 512

function parse(payload: Uint8Array): Record<string, unknown> | null {
  if (!decoder || payload.byteLength === 0 || payload.byteLength > MAX_PAYLOAD) return null
  try {
    const value: unknown = JSON.parse(decoder.decode(payload))
    return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null
  } catch {
    return null
  }
}

export function encodeMessage(body: object): Uint8Array<ArrayBuffer> {
  return (encoder ?? new TextEncoder()).encode(JSON.stringify(body))
}

export function parseHand(payload: Uint8Array): HandMessage | null {
  const v = parse(payload)
  if (!v || typeof v.raised !== 'boolean') return null
  const at = typeof v.at === 'number' && Number.isFinite(v.at) ? v.at : Date.now()
  return { raised: v.raised, at }
}

/** Only the six reactions of the popover pass; a stranger's arbitrary text never reaches the screen. */
export function parseReaction(payload: Uint8Array): ReactionMessage | null {
  const v = parse(payload)
  if (!v || typeof v.emoji !== 'string' || !REACTION_SET.has(v.emoji)) return null
  return { emoji: v.emoji }
}

export function parseShare(payload: Uint8Array): ShareMessage | null {
  const v = parse(payload)
  return v && v.action === 'start' ? { action: 'start' } : null
}

/**
 * The snapshot before a room exists. It lives here, not in `call-room`, so the app shell can
 * read it without importing the LiveKit SDK that `call-room` pulls in.
 */
export const EMPTY_SNAPSHOT: CallSnapshot = {
  link: 'connecting', end: null, participants: [], presenterId: null, speakerId: null, audio: [],
  audioBlocked: false, micOn: false, camOn: false, sharing: false, handUp: false, reactions: [],
  mediaProblem: null, notice: null, peak: 0, facing: 'user',
}
