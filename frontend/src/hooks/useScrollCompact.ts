import { useCallback, useEffect, useRef, useState } from 'react'

/** Past this scroll depth (px), scrolling down folds the bar. */
export const COMPACT_AFTER = 24
/** Scrolling up by more than this (px) at once opens it again. */
const REOPEN_BY = 4

/**
 * Whether the bottom bar should fold to icons, from how the screen under it is scrolled. The screens
 * scroll in elements of their own, and `scroll` does not bubble, so one capturing listener on `scope`
 * (the element holding the screen) sees them all; the one that scrolled last is the scroll root.
 *
 * Folds on scrolling down past 24px, opens on a little scrolling up or back at the top, and starts open
 * again whenever `resetKey` changes (another screen). Never folds when `enabled` is false (reduced motion).
 * `scrollToTop` returns true when there was nothing to scroll, so a second press can mean something else.
 */
export function useScrollCompact(scope: HTMLElement | null, resetKey: string, enabled = true) {
  const [compact, setCompact] = useState(false)
  const root = useRef<HTMLElement | null>(null)
  /** Where each scroller was last seen, so a sideways scroll (a row of chips) is told from a vertical one. */
  const tops = useRef(new WeakMap<HTMLElement, number>())

  useEffect(() => {
    setCompact(false)
    root.current = null
    tops.current = new WeakMap()
  }, [resetKey])

  useEffect(() => {
    if (!scope) return
    const onScroll = (e: Event) => {
      const el = e.target
      if (!(el instanceof HTMLElement) || !scope.contains(el)) return
      const top = el.scrollTop
      // A scroller seen for the first time starts from the top, so its first move already has a direction.
      const delta = top - (tops.current.get(el) ?? 0)
      if (delta === 0) return
      tops.current.set(el, top)
      root.current = el
      if (!enabled || top <= COMPACT_AFTER) setCompact(false)
      else if (delta > 0) setCompact(true)
      else if (delta < -REOPEN_BY) setCompact(false)
    }
    scope.addEventListener('scroll', onScroll, true)
    return () => scope.removeEventListener('scroll', onScroll, true)
  }, [scope, enabled])

  const scrollToTop = useCallback((smooth: boolean): boolean => {
    const el = root.current?.isConnected ? root.current : scope?.querySelector<HTMLElement>('.overflow-y-auto')
    if (!el || el.scrollTop <= 0) return true
    el.scrollTo?.({ top: 0, behavior: smooth ? 'smooth' : 'auto' })
    return false
  }, [scope])

  return { compact: enabled && compact, scrollToTop }
}
