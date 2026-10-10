import { describe, expect, it } from 'vitest'
import { tabOf, tabSearch, validateSettingsSearch } from './settings-search'

describe('settings search', () => {
  it('opens Hồ sơ unless a known tab is named', () => {
    expect(tabOf(validateSettingsSearch({}))).toBe('ho-so')
    expect(tabOf(validateSettingsSearch({ tab: 'giao-dien' }))).toBe('giao-dien')
    expect(tabOf(validateSettingsSearch({ tab: 'workspace' }))).toBe('workspace')
    expect(tabOf(validateSettingsSearch({ tab: 'admin' }))).toBe('ho-so')
    expect(tabOf(validateSettingsSearch({ tab: 7 }))).toBe('ho-so')
  })

  it('keeps the workspace across tabs and leaves no tab for Hồ sơ', () => {
    expect(tabSearch({ ws: 'w1', tab: 'giao-dien' }, 'ho-so')).toEqual({ ws: 'w1' })
    expect(tabSearch({ ws: 'w1' }, 'workspace')).toEqual({ ws: 'w1', tab: 'workspace' })
  })
})
