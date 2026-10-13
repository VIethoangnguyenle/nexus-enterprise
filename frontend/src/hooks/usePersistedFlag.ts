import { useCallback, useState } from 'react'

function read(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

/**
 * A switch the person set and the device remembers (the unread-only filter of
 * the phone lists). Storage can be blocked or full; the switch then still works
 * for this visit and is simply forgotten.
 */
export function usePersistedFlag(key: string): [boolean, (on: boolean) => void] {
  const [state, setState] = useState(() => ({ key, on: read(key) }))
  // A new key (another person on this device) starts from what that key holds.
  if (state.key !== key) setState({ key, on: read(key) })

  const set = useCallback((on: boolean) => {
    setState({ key, on })
    try {
      localStorage.setItem(key, on ? '1' : '0')
    } catch {
      // Not remembering is the only consequence.
    }
  }, [key])

  return [state.key === key ? state.on : read(key), set]
}
