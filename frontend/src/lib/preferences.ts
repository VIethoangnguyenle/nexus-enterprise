/**
 * How this person likes the app to look on this device: theme, motion and the
 * sidebar. Kept in localStorage under the user, so two people sharing a browser
 * keep their own. Nothing here is sent to the server.
 */
export type ThemePref = 'system' | 'light' | 'dark'
export type MotionPref = 'system' | 'reduce'

export type ContactsView = 'table' | 'cards'

export interface Preferences {
  theme: ThemePref
  motion: MotionPref
  sidebarCollapsed: boolean
  /** How Danh bạ lays people out. */
  contactsView: ContactsView
}

export const DEFAULT_PREFERENCES: Preferences = {
  theme: 'system', motion: 'system', sidebarCollapsed: false, contactsView: 'table',
}

const THEMES: readonly ThemePref[] = ['system', 'light', 'dark']
const MOTIONS: readonly MotionPref[] = ['system', 'reduce']

const KEY_PREFIX = 'ngac-prefs'
/** The last preferences applied on this device, read before anyone has signed in. */
const LAST_KEY = `${KEY_PREFIX}:last`

const keyFor = (userId?: string) => (userId ? `${KEY_PREFIX}:${userId}` : LAST_KEY)

/** Reads stored preferences, ignoring anything malformed or unknown. */
export function parsePreferences(raw: string | null): Preferences {
  if (!raw) return DEFAULT_PREFERENCES
  try {
    const v = JSON.parse(raw) as Partial<Record<keyof Preferences, unknown>>
    return {
      theme: THEMES.includes(v.theme as ThemePref) ? (v.theme as ThemePref) : DEFAULT_PREFERENCES.theme,
      motion: MOTIONS.includes(v.motion as MotionPref) ? (v.motion as MotionPref) : DEFAULT_PREFERENCES.motion,
      sidebarCollapsed: v.sidebarCollapsed === true,
      contactsView: v.contactsView === 'cards' ? 'cards' : 'table',
    }
  } catch {
    return DEFAULT_PREFERENCES
  }
}

export function loadPreferences(userId?: string): Preferences {
  try {
    return parsePreferences(localStorage.getItem(keyFor(userId)))
  } catch {
    return DEFAULT_PREFERENCES
  }
}

export function savePreferences(userId: string | undefined, prefs: Preferences): void {
  try {
    const raw = JSON.stringify(prefs)
    if (userId) localStorage.setItem(keyFor(userId), raw)
    localStorage.setItem(LAST_KEY, raw)
  } catch {
    // Storage may be full or blocked; the choice still holds for this session.
  }
}

/** The theme the page shows: "system" follows the device. */
export function resolveTheme(pref: ThemePref, systemDark: boolean): 'light' | 'dark' {
  if (pref === 'system') return systemDark ? 'dark' : 'light'
  return pref
}

const DARK_QUERY = '(prefers-color-scheme: dark)'
export const systemPrefersDark = () => !!window.matchMedia?.(DARK_QUERY).matches

/**
 * Puts the preferences on the document. Light is the absence of `data-theme`
 * (DESIGN.md: light is the default and dark opts in); reduced motion is
 * `data-motion="reduce"`, which index.css treats like the OS setting. The
 * colour change is applied at once, with no cross-fade, so nothing flashes.
 */
export function applyPreferences(prefs: Preferences, systemDark = systemPrefersDark()): void {
  const root = document.documentElement
  if (resolveTheme(prefs.theme, systemDark) === 'dark') root.dataset.theme = 'dark'
  else delete root.dataset.theme
  if (prefs.motion === 'reduce') root.dataset.motion = 'reduce'
  else delete root.dataset.motion
}

// ---- a tiny store, so components and the motion presets read one value ----

// Before anyone is known the device's last choice applies, so a reload does not
// flash light before the signed-in person's own choice is read.
let current: Preferences = typeof localStorage === 'undefined' ? DEFAULT_PREFERENCES : loadPreferences()
let owner: string | undefined
const listeners = new Set<() => void>()

export const getPreferences = () => current

export function subscribePreferences(cb: () => void): () => void {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

function publish(next: Preferences) {
  current = next
  listeners.forEach((cb) => cb())
}

/** Switches to a person's stored preferences (or the device's last, when nobody is known). */
export function switchPreferencesTo(userId?: string): void {
  owner = userId
  const stored = loadPreferences(userId)
  if (JSON.stringify(stored) !== JSON.stringify(current)) publish(stored)
  applyPreferences(stored)
}

/** Changes some preferences, keeps them, and applies them. */
export function updatePreferences(patch: Partial<Preferences>): void {
  const next = { ...current, ...patch }
  savePreferences(owner, next)
  applyPreferences(next)
  publish(next)
}

if (typeof document !== 'undefined') applyPreferences(current)
