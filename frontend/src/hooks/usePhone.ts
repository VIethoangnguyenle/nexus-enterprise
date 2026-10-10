import { useSyncExternalStore } from 'react'

const PHONE = '(max-width: 767.98px)'

const subscribe = (cb: () => void) => {
  const mq = window.matchMedia?.(PHONE)
  mq?.addEventListener('change', cb)
  return () => mq?.removeEventListener('change', cb)
}

const snapshot = () => !!window.matchMedia?.(PHONE).matches

/**
 * True below 768px (DESIGN.md §5, phone). For the few places where the phone
 * gets a different control rather than a different layout, such as the
 * permission switches that replace the matrix.
 */
export function usePhone(): boolean {
  return useSyncExternalStore(subscribe, snapshot, () => false)
}
