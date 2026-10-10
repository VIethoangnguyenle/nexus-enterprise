import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  DEFAULT_PREFERENCES, applyPreferences, getPreferences, loadPreferences, parsePreferences, resolveTheme,
  savePreferences, switchPreferencesTo, updatePreferences,
} from './preferences'

const root = document.documentElement

beforeEach(() => {
  localStorage.clear()
  delete root.dataset.theme
  delete root.dataset.motion
  switchPreferencesTo(undefined)
})
afterEach(() => localStorage.clear())

describe('parsePreferences', () => {
  it('falls back to the defaults for nothing, junk, or unknown values', () => {
    expect(parsePreferences(null)).toEqual(DEFAULT_PREFERENCES)
    expect(parsePreferences('{not json')).toEqual(DEFAULT_PREFERENCES)
    expect(parsePreferences(JSON.stringify({ theme: 'neon', motion: 'wild', sidebarCollapsed: 'yes' }))).toEqual(DEFAULT_PREFERENCES)
  })

  it('remembers the card layout of Danh bạ and ignores anything else', () => {
    expect(parsePreferences(JSON.stringify({ contactsView: 'cards' })).contactsView).toBe('cards')
    expect(parsePreferences(JSON.stringify({ contactsView: 'mosaic' })).contactsView).toBe('table')
  })

  it('keeps valid values', () => {
    const raw = JSON.stringify({ theme: 'dark', motion: 'reduce', sidebarCollapsed: true })
    expect(parsePreferences(raw)).toEqual({ theme: 'dark', motion: 'reduce', sidebarCollapsed: true, contactsView: 'table' })
  })
})

describe('resolveTheme', () => {
  it('follows the device only for "system"', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })
})

describe('applyPreferences', () => {
  it('marks dark and reduced motion on the document, and removes them again', () => {
    applyPreferences({ ...DEFAULT_PREFERENCES, theme: 'dark', motion: 'reduce' }, false)
    expect(root.dataset.theme).toBe('dark')
    expect(root.dataset.motion).toBe('reduce')

    applyPreferences({ ...DEFAULT_PREFERENCES, theme: 'light' }, true)
    expect(root.dataset.theme).toBeUndefined()
    expect(root.dataset.motion).toBeUndefined()
  })

  it('"system" takes the device theme', () => {
    applyPreferences({ ...DEFAULT_PREFERENCES, theme: 'system' }, true)
    expect(root.dataset.theme).toBe('dark')
  })
})

describe('per-person storage', () => {
  it('keeps each person their own choice on a shared device', () => {
    savePreferences('user-a', { ...DEFAULT_PREFERENCES, theme: 'dark' })
    savePreferences('user-b', { ...DEFAULT_PREFERENCES, theme: 'light' })

    expect(loadPreferences('user-a').theme).toBe('dark')
    expect(loadPreferences('user-b').theme).toBe('light')
    expect(loadPreferences('user-c')).toEqual(DEFAULT_PREFERENCES)
  })

  it('updatePreferences stores under the current person, applies, and notifies', () => {
    switchPreferencesTo('user-a')
    updatePreferences({ theme: 'dark' })

    expect(getPreferences().theme).toBe('dark')
    expect(root.dataset.theme).toBe('dark')
    expect(loadPreferences('user-a').theme).toBe('dark')
    expect(loadPreferences('user-b').theme).toBe('system')
  })

  it('switching person swaps the applied preferences', () => {
    savePreferences('user-a', { ...DEFAULT_PREFERENCES, theme: 'dark' })
    switchPreferencesTo('user-a')
    expect(root.dataset.theme).toBe('dark')

    switchPreferencesTo('user-b')
    expect(getPreferences().theme).toBe('system')
    expect(root.dataset.theme).toBeUndefined()
  })
})
