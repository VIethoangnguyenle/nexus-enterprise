import type { CSSProperties } from 'react'

/** Colour of a calendar: a person hue index 0..7, or this for the accent (workspace calendars). */
export const ACCENT_COLOR = -1
/** The eight choices of the colour menu, as hue indexes. */
export const CALENDAR_HUES = [0, 1, 2, 3, 4, 5, 6, 7] as const

/** The token pair of a calendar colour: foreground and its wash. Never a literal colour. */
export function colorTokens(color: number): { fg: string; wash: string } {
  if (!Number.isInteger(color) || color < 0 || color > 7) return { fg: 'var(--color-accent)', wash: 'var(--color-accent-wash)' }
  return { fg: `var(--color-person-${color + 1})`, wash: `var(--color-person-${color + 1}-wash)` }
}

/** Inline custom properties `--c` (fill) and `--w` (wash) that the event blocks read through `bg-(--c)` utilities. */
export function colorStyle(color: number): CSSProperties {
  const { fg, wash } = colorTokens(color)
  return { ['--c' as string]: fg, ['--w' as string]: wash }
}

/** Plain-language name of a colour in the menu (one of the eight hues, in the order of DESIGN.md §2). */
export const HUE_NAMES = ['Cam đất', 'Vàng rêu', 'Lá', 'Biển', 'Lam', 'Tím', 'Mận', 'Hồng'] as const
export const colorName = (color: number) => (color >= 0 && color <= 7 ? HUE_NAMES[color]! : 'Xanh ngọc')
