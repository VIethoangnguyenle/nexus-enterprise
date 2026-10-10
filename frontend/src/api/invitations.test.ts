import { beforeEach, describe, expect, it, vi } from 'vitest'
import { invitationsApi } from './invitations'

/** Pins the invitee's routes; none of them takes a person or an address in its body. */
const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))
beforeEach(() => apiFetch.mockReset().mockResolvedValue({}))

const last = () => {
  const [url, init] = apiFetch.mock.calls[apiFetch.mock.calls.length - 1] as [string, RequestInit | undefined]
  return { url: `/api${url}`, method: init?.method ?? 'GET', body: init?.body }
}

describe('invitee requests', () => {
  it('lists, accepts and declines by invitation only', async () => {
    await invitationsApi.listMine()
    expect(last()).toEqual({ url: '/api/invitations', method: 'GET', body: undefined })
    await invitationsApi.accept('i1')
    expect(last()).toEqual({ url: '/api/invitations/i1/accept', method: 'POST', body: undefined })
    await invitationsApi.decline('i1')
    expect(last()).toEqual({ url: '/api/invitations/i1/decline', method: 'POST', body: undefined })
  })
})
