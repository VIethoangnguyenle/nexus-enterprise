import type { CSSProperties } from 'react'

/** One of the eight person hues in DESIGN.md §2. */
export type PersonHue = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8

/**
 * Deterministic hue for a person or a space.
 *
 * The key is a stable identifier (user id, channel id). It is hashed on the
 * client and only the resulting 1..8 ever reaches the DOM, so the identifier is
 * never rendered. FNV-1a over UTF-16 code units: tiny, fast, and spreads short
 * keys well enough that neighbouring ids rarely share a colour.
 */
export function personHue(key: string): PersonHue {
  let h = 0x811c9dc5
  for (let i = 0; i < key.length; i++) {
    h ^= key.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  // Final avalanche so keys that differ only in the last character still move.
  h ^= h >>> 16
  h = Math.imul(h, 0x45d9f3b)
  h ^= h >>> 16
  return (((h >>> 0) % 8) + 1) as PersonHue
}

/**
 * Inline custom properties for a person hue: `--p` (fill) and `--pw` (wash).
 * Components read them through `bg-(--p)` style utilities and the realtime
 * keyframes in index.css.
 */
export function personStyle(key: string): CSSProperties {
  const n = personHue(key)
  return {
    ['--p' as string]: `var(--color-person-${n})`,
    ['--pw' as string]: `var(--color-person-${n}-wash)`,
  }
}
