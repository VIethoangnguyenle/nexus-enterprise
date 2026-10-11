import { useMotionPresets } from '../../lib/motion'
import './nexus-mark.css'

export interface NexusMarkProps {
  /** Edge of the square, in px. At 24 and under the heavier small drawing is used. */
  size?: number
  /** Build the mark up node by node (once per browser session on the sign-in screens). */
  assemble?: boolean
  /** `on-accent` draws one colour, for use on the accent fill (an app icon, a filled chip). */
  tone?: 'default' | 'on-accent'
  /** True when "Nexus" is written beside it: the mark is then skipped by screen readers. */
  decorative?: boolean
  className?: string
}

/** Largest size that still gets the small drawing (design/mockups/logo.html, rules of use). */
const SMALL_AT = 24

/*
 * Direction A, "Lối đi": the letter N as one path through four nodes on a 48 grid.
 * Stroke 6, node radius 5, the last node (the place you reach) radius 5.5 in person
 * colour 1. Edges are drawn as bars, not one stroked path, so each can be scaled in
 * on its own when the mark assembles. The bars start under their node, which hides
 * the butt ends.
 */
const EDGES = [
  { transform: 'translate(12 37) rotate(-90)', length: 26 },
  { transform: 'translate(12 11) rotate(47.29)', length: 35.38 },
  { transform: 'translate(36 37) rotate(-90)', length: 26 },
]
const NODES = [
  { cx: 12, cy: 37, r: 5 },
  { cx: 12, cy: 11, r: 5 },
  { cx: 36, cy: 37, r: 5 },
]

export function NexusMark({ size = 32, assemble = false, tone = 'default', decorative = false, className = '' }: NexusMarkProps) {
  const { reduced } = useMotionPresets()
  const small = size <= SMALL_AT
  const label = decorative ? { 'aria-hidden': true as const } : { role: 'img' as const, 'aria-label': 'Nexus' }

  return (
    <svg
      {...label}
      width={size}
      height={size}
      viewBox="0 0 48 48"
      focusable="false"
      className={`nexus-mark ${className}`}
      data-tone={tone}
      data-assemble={assemble && !small ? 'true' : undefined}
      data-reduced={reduced ? 'true' : undefined}
    >
      {small ? (
        <>
          <path d="M12 37V11L36 37V11" fill="none" stroke="currentColor" strokeWidth="8" strokeLinecap="round" strokeLinejoin="round" className="nm-line" />
          <circle data-nexus-node="end" className="nm-end" cx="36" cy="11" r="6.5" />
        </>
      ) : (
        <>
          {EDGES.map((e, i) => (
            <g key={i} transform={e.transform}>
              <rect data-nexus-edge={i + 1} className="nm-edge" x="0" y="-3" width={e.length} height="6" />
            </g>
          ))}
          {NODES.map((n, i) => (
            <circle key={i} data-nexus-node={i + 1} className="nm-node" cx={n.cx} cy={n.cy} r={n.r} />
          ))}
          <circle data-nexus-node="end" className="nm-node nm-end" cx="36" cy="11" r="5.5" />
        </>
      )}
    </svg>
  )
}
