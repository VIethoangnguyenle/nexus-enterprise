import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { explain, FOLDER_HAS_DOCUMENTS, QUOTA_EXCEEDED, isForbidden, reasonOf, statusOf } from './errors'

const fail = (status: number, body?: unknown) => new ApiError('server text with id 123', status, body)

describe('errors', () => {
  it('reads status and reason only from an ApiError', () => {
    expect(statusOf(fail(409))).toBe(409)
    expect(statusOf(new Error('x'))).toBeUndefined()
    expect(reasonOf(fail(409, { reason: 'r' }))).toBe('r')
    expect(reasonOf(fail(409, { reason: 5 }))).toBeUndefined()
    expect(reasonOf(fail(409))).toBeUndefined()
    expect(reasonOf('nope')).toBeUndefined()
  })

  it('isForbidden is true for 403 only', () => {
    expect(isForbidden(fail(403))).toBe(true)
    expect(isForbidden(fail(404))).toBe(false)
    expect(isForbidden(undefined)).toBe(false)
  })

  it.each([
    [403, /chưa có quyền xoá/],
    [404, /không còn nữa/],
    [409, /vừa được người khác thay đổi/],
    [400, /chưa hợp lệ/],
    [500, /Máy chủ đang gặp sự cố/],
    [503, /Máy chủ đang gặp sự cố/],
  ])('explains a %i in the action named', (status, re) => {
    const text = explain(fail(status), 'xoá')
    expect(text).toMatch(re)
    expect(text).toContain('xoá')
  })

  it('names the documents in the folder for the folder_has_documents conflict', () => {
    expect(explain(fail(409, { reason: FOLDER_HAS_DOCUMENTS }), 'xoá')).toMatch(/còn văn bản/)
  })

  it('says the storage is full for a 413 quota refusal, not a network problem', () => {
    const text = explain(fail(413, { reason: QUOTA_EXCEEDED }), 'tải tệp lên')
    expect(text).toBe('Chưa tải tệp lên được vì kho tài liệu của workspace đã đầy. Xoá bớt tệp hoặc nhờ quản trị viên tăng dung lượng.')
    expect(explain(fail(413), 'tải tệp lên')).toMatch(/đã đầy/)
  })

  it('falls back to the network sentence and never echoes server text', () => {
    const text = explain(new Error('boom'), 'lưu')
    expect(text).toMatch(/kết nối mạng/)
    expect(explain(fail(500), 'lưu')).not.toContain('123')
  })
})
