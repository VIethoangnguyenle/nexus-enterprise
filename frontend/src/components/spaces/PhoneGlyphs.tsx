/**
 * Icons the phone lists draw that the icon set has no match for
 * (design/mockups/chat-mobile-home.html: `i-thread`, `i-unread`). 1.75 stroke on
 * a 24 grid, like every other icon.
 */
interface GlyphProps {
  size?: number
  className?: string
}

const base = {
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.75,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
  'aria-hidden': true,
} as const

/** A reply branching off a line: the thread. */
export function ThreadGlyph({ size = 16, className }: GlyphProps) {
  return (
    <svg {...base} width={size} height={size} className={className}>
      <path d="M7 4v9a4 4 0 0 0 4 4h9" />
      <path d="m16 13 4 4-4 4" />
    </svg>
  )
}

/** A speech bubble with a dot: show only what is unread. */
export function UnreadGlyph({ size = 20, className }: GlyphProps) {
  return (
    <svg {...base} width={size} height={size} className={className}>
      <path d="M13 4H5a2 2 0 0 0-2 2v14l4-4h12a2 2 0 0 0 2-2v-4" />
      <circle cx="19" cy="5" r="2.5" fill="currentColor" stroke="none" />
    </svg>
  )
}
