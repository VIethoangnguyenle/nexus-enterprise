import { describe, expect, it } from 'vitest'
import { FIELD_MAX, NAME_MAX, profileChanges, profileOf, validateProfile } from './settings-model'

const form = { displayName: 'Lê Thị Hoa', title: 'Kế toán trưởng', location: '' }

describe('profileOf', () => {
  it('never puts a username in the name field, and starts empty for nobody', () => {
    expect(profileOf({ display_name: '', title: '', location: '' }).displayName).toBe('')
    expect(profileOf(undefined)).toEqual({ displayName: '', title: '', location: '' })
  })
})

describe('validateProfile', () => {
  it('accepts a name with accents and optional empty fields', () => {
    expect(validateProfile(form)).toEqual({})
  })

  it('asks for a name, and says how long it may be', () => {
    expect(validateProfile({ ...form, displayName: '   ' }).displayName).toMatch(/Nhập tên/)
    expect(validateProfile({ ...form, displayName: 'a'.repeat(NAME_MAX + 1) }).displayName).toMatch(String(NAME_MAX))
    expect(validateProfile({ ...form, displayName: 'a'.repeat(NAME_MAX) })).toEqual({})
  })

  it('counts characters, not bytes', () => {
    expect(validateProfile({ ...form, title: 'ế'.repeat(FIELD_MAX) })).toEqual({})
    expect(validateProfile({ ...form, title: 'ế'.repeat(FIELD_MAX + 1) }).title).toBeDefined()
  })

  it('refuses control characters', () => {
    expect(validateProfile({ ...form, location: 'Hà\u0000 Nội' }).location).toBeDefined()
  })
})

describe('profileChanges', () => {
  it('is null when nothing changed, even if only spaces were added', () => {
    expect(profileChanges(form, { ...form, displayName: ' Lê Thị Hoa ' })).toBeNull()
  })

  it('sends only the fields that changed', () => {
    expect(profileChanges(form, { ...form, title: 'Giám đốc' })).toEqual({ title: 'Giám đốc' })
    expect(profileChanges(form, { ...form, location: ' Hà Nội ', displayName: 'Hoa' }))
      .toEqual({ display_name: 'Hoa', location: 'Hà Nội' })
  })

  it('can clear an optional field', () => {
    expect(profileChanges(form, { ...form, title: '' })).toEqual({ title: '' })
  })
})
