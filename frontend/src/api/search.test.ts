import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch } from './client'
import { searchApprovals, searchDrive } from './search'

vi.mock('./client', async (orig) => ({ ...(await orig<typeof import('./client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

afterEach(() => api.mockReset())

describe('searchDrive', () => {
  it('names the workspace, the words and how many', async () => {
    api.mockResolvedValue({ items: [] })
    await searchDrive('ws-1', 'đối soát', 20)
    expect(api).toHaveBeenCalledWith(`/drive/search?workspace_id=ws-1&q=${encodeURIComponent('đối soát').replace(/%20/g, '+')}&limit=20`, { signal: undefined })
  })

  it('passes a refusal on: a member of another workspace learns nothing', async () => {
    api.mockRejectedValue(new ApiError('access denied', 403))
    await expect(searchDrive('ws-other', 'x', 20)).rejects.toMatchObject({ status: 403 })
  })
})

describe('searchApprovals', () => {
  it('sends the words with only the filters that are set', async () => {
    api.mockResolvedValue({ items: [], total: 0 })
    await searchApprovals('laptop', { status: 'pending', template_id: '' }, 3)
    expect(api).toHaveBeenCalledWith('/approval/search?q=laptop&limit=3&status=pending', { signal: undefined })
  })

  it('reads a missing total as the number of items', async () => {
    api.mockResolvedValue({ items: [{ id: 'r1' }] })
    await expect(searchApprovals('x', {}, 3)).resolves.toMatchObject({ total: 1 })
  })

  it.each([404, 501])('answers nothing found, not an error, while the route is not there (%i)', async (status) => {
    api.mockRejectedValue(new ApiError('not found', status))
    await expect(searchApprovals('x', {}, 3)).resolves.toEqual({ items: [], total: 0 })
  })

  it('passes a refusal and a failure on', async () => {
    api.mockRejectedValue(new ApiError('forbidden', 403))
    await expect(searchApprovals('x', {}, 3)).rejects.toMatchObject({ status: 403 })
    api.mockRejectedValue(new ApiError('internal error', 500))
    await expect(searchApprovals('x', {}, 3)).rejects.toMatchObject({ status: 500 })
  })
})
