import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { AVATAR_MAX_BYTES, avatarErrorMessage, validateAvatarFile } from './avatar-file'

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
