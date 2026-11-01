import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

const state = { accessToken: 'tok' as string | null, setAccessToken: vi.fn(), logout: vi.fn() }
vi.mock('../stores/auth.store', () => ({ useAuthStore: { getState: () => state } }))

const { profileApi } = await import('./profile')

beforeEach(() => {
  vi.clearAllMocks()
  mockFetch.mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve({ status: 'updated' }) })
})

const sent = () => {
  const [url, init] = mockFetch.mock.calls[0]! as [string, RequestInit]
  return { url, method: init.method, body: JSON.parse(String(init.body)) }
}

describe('profileApi.update', () => {
  it('names the workspace the title is for, so it changes only there', async () => {
    await profileApi.update({ title: 'Kế toán trưởng' }, 'w 1')
    expect(sent()).toEqual({ url: '/api/me/profile?workspace=w%201', method: 'PATCH', body: { title: 'Kế toán trưởng' } })
  })

  it('without a workspace writes the account default', async () => {
    await profileApi.update({ title: 'Chuyên viên' })
    expect(sent()).toEqual({ url: '/api/me/profile', method: 'PATCH', body: { title: 'Chuyên viên' } })
  })
})
