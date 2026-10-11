import { useEffect, useState, type CSSProperties } from 'react'
import { useMotionPresets } from '../../lib/motion'
import { GRAPH_SETTLE_SECONDS } from './auth-motion'

/*
 * A small attribute graph, drawn the way the product thinks: rings are
 * attributes, filled dots are people in their own colours, lines are the
 * assignments between them. Pure decoration (aria-hidden) for the brand pane.
 * Points are on a 600 x 720 canvas; people use person hues 1 to 8.
 */
type Point = readonly [x: number, y: number]
const ATTRIBUTES: readonly Point[] = [[360, 90], [540, 70], [420, 250], [480, 380], [580, 430], [380, 470], [300, 590], [440, 640]]
const PEOPLE: readonly (readonly [x: number, y: number, hue: number])[] = [
  [470, 150, 5], [560, 260, 3], [330, 340, 1], [520, 540, 6], [570, 650, 8], [250, 200, 4],
]
/** Edges as pairs of points drawn from the lists above (attributes first, then people). */
const EDGES: readonly (readonly [number, number])[] = [
  [0, 8], [8, 1], [8, 2], [2, 9], [2, 10], [2, 3], [3, 4], [3, 5], [5, 11], [5, 6], [11, 7], [7, 12], [4, 11], [0, 13], [13, 10], [1, 9], [6, 7],
]

const NODES: readonly Point[] = [...ATTRIBUTES, ...PEOPLE.map(([x, y]) => [x, y] as const)]
// The mockup pulls the graph in towards the right edge; kept so the drawing matches it.
const place = ([x, y]: Point): Point => [Math.round(380 + (x - 250) * 0.65), y]

/** One pass, then it stays where it ended; the keyframes and easing are in auth-motion.css. */
const SETTLE: CSSProperties = {
  animationDuration: `${GRAPH_SETTLE_SECONDS}s`,
  animationIterationCount: 1,
  animationFillMode: 'forwards',
}

/** Even-numbered shapes drift on one layer and odd ones on the other, so the two move apart. */
const layer = (i: number) => (i % 2 ? 'l2' : 'l1')

function usePageHidden(): boolean {
  const [hidden, setHidden] = useState(() => typeof document !== 'undefined' && document.hidden)
  useEffect(() => {
    const sync = () => setHidden(document.hidden)
    document.addEventListener('visibilitychange', sync)
    return () => document.removeEventListener('visibilitychange', sync)
  }, [])
  return hidden
}

/**
 * Drifts for about 30s from the moment it appears and then stands still
 * (auth-motion.css), halts while the tab is hidden so the 30s are 30s of being
 * watched, and is not rendered under reduced motion.
 */
export function AuthGraph() {
  const { reduced } = useMotionPresets()
  const hidden = usePageHidden()
  if (reduced) return null

  const lines: { layer: string; a: Point; b: Point }[] = EDGES.map(([from, to], i) => ({
    layer: layer(i),
    a: place(NODES[from]!),
    b: place(NODES[to]!),
  }))

  return (
    <div className="auth-graph" data-auth-graph aria-hidden="true" data-paused={hidden ? 'true' : undefined}>
      <svg viewBox="0 0 600 720" preserveAspectRatio="xMaxYMid slice">
        {(['l1', 'l2'] as const).map((l) => (
          <g key={l} className={l} style={SETTLE}>
            {lines.filter((e) => e.layer === l).map((e, i) => (
              <line key={i} className="ed" x1={e.a[0]} y1={e.a[1]} x2={e.b[0]} y2={e.b[1]} />
            ))}
            {ATTRIBUTES.map((p, i) => (layer(i) === l ? (
              <circle key={`a${i}`} className="oa" cx={place(p)[0]} cy={place(p)[1]} r={7} />
            ) : null))}
            {PEOPLE.map((p, i) => (layer(i + ATTRIBUTES.length) === l ? (
              <circle key={`p${i}`} className="u" cx={place([p[0], p[1]])[0]} cy={place([p[0], p[1]])[1]} r={5}
                style={{ fill: `var(--color-person-${p[2]})` }} />
            ) : null))}
          </g>
        ))}
      </svg>
    </div>
  )
}
