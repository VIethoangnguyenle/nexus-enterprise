import { useSyncExternalStore } from 'react'

const PHONE = '(max-width: 767.98px)'
/** Where the sidebar is not shown (Tailwind `lg`), so the phone bars take over. */
const BOTTOM_BAR = '(max-width: 1023.98px)'

const subscriberFor = (query: string) => (cb: () => void) => {
  const mq = window.matchMedia?.(query)
  mq?.addEventListener('change', cb)
  return () => mq?.removeEventListener('change', cb)
}
const snapshotFor = (query: string) => () => !!window.matchMedia?.(query).matches

const subscribe = subscriberFor(PHONE)
const snapshot = snapshotFor(PHONE)
const subscribeBar = subscriberFor(BOTTOM_BAR)
const snapshotBar = snapshotFor(BOTTOM_BAR)

/**
 * True below 768px (DESIGN.md §5, phone). For the few places where the phone
 * gets a different control rather than a different layout, such as the
 * permission switches that replace the matrix.
 */
export function usePhone(): boolean {
  return useSyncExternalStore(subscribe, snapshot, () => false)
}

/**
 * True below 1024px: the widths where the sidebar is hidden and the phone top
 * and bottom bars carry navigation (DESIGN.md §5, until the 768-1023 rail
 * exists). Components that only matter there skip their work above it.
 */
export function useBottomBarLayout(): boolean {
  return useSyncExternalStore(subscribeBar, snapshotBar, () => false)
}
