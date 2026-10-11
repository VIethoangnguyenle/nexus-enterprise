import type { TargetAndTransition } from 'motion/react'
import { DURATION, EASE } from './motion'

export interface ItemMotion {
  initial: TargetAndTransition | false
  animate: TargetAndTransition
  exit: TargetAndTransition
}

/**
 * How a card or row of Được nhắc tới / Đã gắn sao moves (design/mockups/shortcuts.html):
 * a new mention grows in over 220ms, an unstarred item collapses over 220ms.
 * Under reduced motion height never animates; both are a fade of at most 120ms.
 * `enter` is false for items that are simply there (the first page, "Tải thêm").
 */
export function feedItemMotion(reduced: boolean, enter: boolean): ItemMotion {
  if (reduced) {
    const fade = { duration: DURATION.reducedFade }
    return {
      initial: enter ? { opacity: 0 } : false,
      animate: { opacity: 1, transition: fade },
      exit: { opacity: 0, transition: fade },
    }
  }
  return {
    initial: enter ? { opacity: 0, height: 0 } : false,
    animate: { opacity: 1, height: 'auto', transition: { duration: DURATION.base, ease: EASE.outExpo } },
    exit: { opacity: 0, height: 0, transition: { duration: DURATION.base, ease: EASE.in } },
  }
}
