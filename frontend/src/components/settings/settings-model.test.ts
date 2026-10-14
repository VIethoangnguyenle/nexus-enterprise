import { describe, expect, it } from 'vitest'
import { ApiError } from '../../api/client'
import {
  AVATAR_MAX_BYTES, FIELD_MAX, NAME_MAX, avatarErrorMessage, profileChanges, profileOf, validateAvatarFile, validateProfile,
} from './settings-model'

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

describe('validateAvatarFile', () => {
  const file = (type: string, size: number) => new File([new Uint8Array(size)], 'anh', { type })

  it('takes a JPG or PNG up to 5 MB', () => {
    expect(validateAvatarFile(file('image/jpeg', 1024 * 1024))).toBeNull()
    expect(validateAvatarFile(file('image/png', AVATAR_MAX_BYTES))).toBeNull()
  })

  it('refuses any other kind of file, before anything is sent', () => {
    expect(validateAvatarFile(file('image/gif', 10))).toMatch(/JPG hoặc PNG/)
    expect(validateAvatarFile(file('image/svg+xml', 10))).toMatch(/JPG hoặc PNG/)
    expect(validateAvatarFile(file('', 10))).toMatch(/JPG hoặc PNG/)
  })

  it('refuses a file over 5 MB', () => {
    expect(validateAvatarFile(file('image/png', AVATAR_MAX_BYTES + 1))).toMatch(/5 MB/)
  })
})

describe('avatarErrorMessage', () => {
  const refused = (status: number, code?: string) => new ApiError('x', status, code ? { code } : {})

  it('says in words what the server refused, never its English text', () => {
    expect(avatarErrorMessage(refused(400, 'avatar_not_image'))).toMatch(/JPG hoặc PNG/)
    expect(avatarErrorMessage(refused(413, 'avatar_too_large'))).toMatch(/5 MB/)
    expect(avatarErrorMessage(refused(400, 'avatar_too_many_pixels'))).toMatch(/kích thước/)
    expect(avatarErrorMessage(refused(503, 'avatar_busy'))).toMatch(/bận/)
    expect(avatarErrorMessage(refused(429, 'rate_limited'))).toMatch(/nhiều lần/)
    expect(avatarErrorMessage(refused(500))).toMatch(/Thử lại/)
    expect(avatarErrorMessage(new TypeError('Failed to fetch'))).toMatch(/kết nối/)
  })
})
