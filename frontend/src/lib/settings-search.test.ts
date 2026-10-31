import { describe, expect, it } from 'vitest'
import { sectionOfTab, tabOf, validateSettingsSearch } from './settings-search'

describe('settings search', () => {
  it('names the section that holds each old tab\'s options', () => {
    expect(sectionOfTab('ho-so')).toBe('account')
    expect(sectionOfTab('workspace')).toBe('workspace')
    expect(sectionOfTab('giao-dien')).toBe('appearance')
    expect(sectionOfTab('thong-bao')).toBe('notifications')
  })

  it('reads Hồ sơ unless a known tab is named', () => {
    expect(tabOf(validateSettingsSearch({}))).toBe('ho-so')
    expect(tabOf(validateSettingsSearch({ tab: 'giao-dien' }))).toBe('giao-dien')
    expect(tabOf(validateSettingsSearch({ tab: 'workspace' }))).toBe('workspace')
    expect(tabOf(validateSettingsSearch({ tab: 'admin' }))).toBe('ho-so')
    expect(tabOf(validateSettingsSearch({ tab: 7 }))).toBe('ho-so')
  })

  it('keeps the workspace from the address', () => {
    expect(validateSettingsSearch({ ws: 'w1', tab: 'thong-bao' })).toEqual({ ws: 'w1', tab: 'thong-bao' })
  })
})
