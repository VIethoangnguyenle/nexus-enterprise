import { useSyncExternalStore } from 'react'

/** The part of the Network Information API this reads; most browsers other than Chromium's have none of it. */
interface NetworkInformationLike extends EventTarget {
  type?: string
  saveData?: boolean
}

const connectionOf = (): NetworkInformationLike | undefined =>
  typeof navigator === 'undefined' ? undefined : (navigator as Navigator & { connection?: NetworkInformationLike }).connection

/**
 * True when the browser says the device is on mobile data (`type` is `cellular`) or the person asked it to save
 * data (`saveData`). A browser that says nothing counts as not metered, so pictures load as they always have.
 */
export function isMeteredConnection(): boolean {
  const c = connectionOf()
  return !!c && (c.type === 'cellular' || c.saveData === true)
}

function subscribe(onChange: () => void): () => void {
  const c = connectionOf()
  if (!c || typeof c.addEventListener !== 'function') return () => {}
  c.addEventListener('change', onChange)
  return () => c.removeEventListener('change', onChange)
}

/** `isMeteredConnection`, kept current as the connection changes (Wi-Fi to mobile data and back). */
export function useMeteredConnection(): boolean {
  return useSyncExternalStore(subscribe, isMeteredConnection, () => false)
}
