import { useCallback, useEffect } from 'react'
import { getPreferences, updatePreferences } from '../lib/preferences'
import { usePreferences } from './usePreferences'

/**
 * Whether the main menu is the 72px rail (true, the default) or the 232px
 * sidebar. The choice lives in the person's own preferences, which are kept in
 * localStorage per user id on this device and never sent to the server
 * (lib/preferences.ts), so it is remembered per person and per device.
 *
 * `shortcut` also binds Ctrl+\ (⌘\ on macOS) anywhere on the page: `true`
 * toggles at once, a function is called instead (the sidebar animates the
 * change). The composer does not use that chord, so it is not held back inside
 * text fields.
 */
export function useSidebarCollapsed({ shortcut = false }: { shortcut?: boolean | (() => void) } = {}) {
  const { prefs } = usePreferences()
  const collapsed = prefs.sidebarCollapsed
  // Reads the stored value at call time, so a toggle scheduled a moment ago still flips the current state.
  const toggle = useCallback(() => updatePreferences({ sidebarCollapsed: !getPreferences().sidebarCollapsed }), [])
  const onShortcut = typeof shortcut === 'function' ? shortcut : toggle

  useEffect(() => {
    if (!shortcut) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '\\' || e.altKey || e.shiftKey) return
      if (e.ctrlKey === e.metaKey) return // exactly one of Ctrl / ⌘
      e.preventDefault()
      onShortcut()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [shortcut, onShortcut])

  return { collapsed, toggle }
}

/** How the chord reads on this device, for tooltips. */
export function sidebarShortcutLabel(): string {
  const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)
  return mac ? '⌘\\' : 'Ctrl+\\'
}
