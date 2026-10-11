import { useEffect, useState } from 'react'
import { DURATION, EASE, type Preset } from '../../lib/motion'

/*
 * Motion of the sign-in screens only: the approved exception in DESIGN.md §7
 * (steps slide 16px, the logo assembles once per session, the brand pane's graph
 * drifts for ~30s and stops). Everything else in the app keeps lib/motion.ts.
 */

/** sessionStorage flag: the logo has already assembled in this browser session. */
export const LOGO_SEEN_KEY = 'nexus:logo-assembled'

/** The graph behind the brand pane drifts this long, once, then stands still. */
export const GRAPH_SETTLE_SECONDS = 30

/** How far a step travels in. The step leaving goes 12px, about 75% of it. */
export const STEP_SHIFT = 16
const STEP_LEAVE = 12

/**
 * Presets for the two sign-in steps. `direction` 1 moves forward (new step from
 * the right), -1 goes back. Reduced motion: a fade, at most 120ms, no transform.
 */
export function stepPresets(reduced: boolean, direction: 1 | -1): Preset {
  if (reduced) {
    return {
      initial: { opacity: 0 },
      animate: { opacity: 1, transition: { duration: DURATION.reducedFade, ease: 'linear' } },
      exit: { opacity: 0, transition: { duration: DURATION.reducedFade, ease: 'linear' } },
    }
  }
  return {
    initial: { opacity: 0, x: STEP_SHIFT * direction },
    animate: { opacity: 1, x: 0, transition: { duration: DURATION.layout, ease: EASE.outExpo } },
    exit: { opacity: 0, x: -STEP_LEAVE * direction, transition: { duration: DURATION.exit, ease: EASE.in } },
  }
}

function logoSeen(): boolean {
  try {
    return sessionStorage.getItem(LOGO_SEEN_KEY) !== null
  } catch {
    // Storage blocked: a mark that is always at rest beats one that replays on every screen.
    return true
  }
}

/**
 * True for the first sign-in screen shown in a browser session, and stays true
 * while that screen lives; false on every later one.
 */
export function useLogoAssembly(): boolean {
  const [assemble] = useState(() => !logoSeen())
  useEffect(() => {
    try {
      sessionStorage.setItem(LOGO_SEEN_KEY, '1')
    } catch {
      /* nothing to remember it with */
    }
  }, [])
  return assemble
}
