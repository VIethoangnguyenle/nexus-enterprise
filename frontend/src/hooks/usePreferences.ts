import { useCallback, useEffect, useSyncExternalStore } from 'react'
import {
  applyPreferences, getPreferences, subscribePreferences, switchPreferencesTo, updatePreferences,
  type Preferences,
} from '../lib/preferences'
import { useAuthStore } from '../stores/auth.store'

/** This person's appearance preferences, and a way to change them. */
export function usePreferences() {
  const prefs = useSyncExternalStore(subscribePreferences, getPreferences, getPreferences)
  const update = useCallback((patch: Partial<Preferences>) => updatePreferences(patch), [])
  return { prefs, update }
}

/**
 * Mounted once in the workspace shell: loads the signed-in person's stored
 * preferences and keeps "Theo hệ thống" in step with the device while it is
 * chosen.
 */
export function usePreferencesSync(): void {
  const userId = useAuthStore((s) => s.user?.id)
  const theme = useSyncExternalStore(subscribePreferences, () => getPreferences().theme)

  useEffect(() => {
    switchPreferencesTo(userId)
  }, [userId])

  useEffect(() => {
    if (theme !== 'system' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => applyPreferences(getPreferences(), mq.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [theme])
}
