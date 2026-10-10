import { beforeEach, describe, expect, it, vi } from 'vitest'

import { assetApi } from './assets'

/**
 * The asset REST handlers bind JSON into structs and ignore any field they do
 * not know, so a misspelt body is not an error: it is a request that does
 * something else (the old client sent `action` where the handler read
 * `to_state`, and `justification` where it read `reason`). These pin the field
 * names and query parameters to the handlers in
 * backend/services/asset/internal/rest/handler.go.
 */
const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))

beforeEach(() => apiFetch.mockReset().mockResolvedValue({}))

const lastCall = () => {
  const [url, init] = apiFetch.mock.calls[apiFetch.mock.calls.length - 1] as [string, RequestInit | undefined]
  return { url, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
}

describe('asset queries', () => {
  it('lists assets with the filters the handler reads, leaving out the ones not set', async () => {
    await assetApi.list('ws1', { type_id: 't1', state: 'available', search: 'dell', limit: 25, offset: 50 })
    const { url, method } = lastCall()
    expect(method).toBe('GET')
    const u = new URL(url, 'http://x')
    expect(u.pathname).toBe('/workspaces/ws1/assets')
    expect(Object.fromEntries(u.searchParams)).toEqual({ type_id: 't1', state: 'available', search: 'dell', limit: '25', offset: '50' })

    await assetApi.list('ws1')
    expect(lastCall().url).toBe('/workspaces/ws1/assets')
    await assetApi.list('ws1', { search: '', state: undefined })
    expect(lastCall().url).toBe('/workspaces/ws1/assets')
  })

  it('reads the summary, the activity and one asset from their routes', async () => {
    await assetApi.getSummary('ws1')
    expect(lastCall().url).toBe('/workspaces/ws1/assets/summary')
    await assetApi.getActivity('ws1', 8)
    expect(lastCall().url).toBe('/workspaces/ws1/assets/activity?limit=8')
    await assetApi.get('a1')
    expect(lastCall().url).toBe('/assets/a1')
    await assetApi.getTransitions('a1')
    expect(lastCall().url).toBe('/assets/a1/transitions')
    await assetApi.getHistory('a1')
    expect(lastCall().url).toBe('/assets/a1/history')
  })

  it('lists requests by status list, mine, and page', async () => {
    await assetApi.listRequests('ws1', { status: 'approved,fulfilled', mine: true, limit: 25, offset: 0 })
    const u = new URL(lastCall().url, 'http://x')
    expect(u.pathname).toBe('/workspaces/ws1/asset-requests')
    expect(Object.fromEntries(u.searchParams)).toEqual({ status: 'approved,fulfilled', mine: 'true', limit: '25', offset: '0' })
    await assetApi.listRequests('ws1')
    expect(lastCall().url).toBe('/workspaces/ws1/asset-requests')
    await assetApi.getRequest('r1')
    expect(lastCall().url).toBe('/asset-requests/r1')
  })
})

describe('asset request bodies', () => {
  it('a new request sends type_id, reason and urgency (not justification)', async () => {
    await assetApi.createRequest('ws1', { type_id: 't1', reason: 'Nhân sự mới', urgency: 'urgent' })
    expect(lastCall()).toEqual({
      url: '/workspaces/ws1/asset-requests', method: 'POST',
      body: { type_id: 't1', reason: 'Nhân sự mới', urgency: 'urgent' },
    })
  })

  it('approve names the asset to hand over, and the comment only when there is one', async () => {
    await assetApi.approveRequest('r1', { asset_id: 'a4', comment: 'Máy mới' })
    expect(lastCall()).toEqual({ url: '/asset-requests/r1/approve', method: 'POST', body: { asset_id: 'a4', comment: 'Máy mới' } })
    await assetApi.approveRequest('r1', {})
    expect(lastCall().body).toEqual({})
  })

  it('reject sends the reason; assign sends the asset', async () => {
    await assetApi.rejectRequest('r1', 'Kho còn máy cũ')
    expect(lastCall()).toEqual({ url: '/asset-requests/r1/reject', method: 'POST', body: { reason: 'Kho còn máy cũ' } })
    await assetApi.assignRequest('r1', 'a4')
    expect(lastCall()).toEqual({ url: '/asset-requests/r1/assign', method: 'POST', body: { asset_id: 'a4' } })
  })
})

describe('asset action bodies', () => {
  it('a lifecycle step sends action (the handler no longer reads a made-up to_state)', async () => {
    await assetApi.transition('a1', 'flag_maintenance', 'Thay pin')
    expect(lastCall()).toEqual({ url: '/assets/a1/transition', method: 'POST', body: { action: 'flag_maintenance', comment: 'Thay pin' } })
    await assetApi.transition('a1', 'retire')
    expect(lastCall().body).toEqual({ action: 'retire' })
  })

  it('a hand-over names the person by assignee_id', async () => {
    await assetApi.handOver('a1', 'u9', 'Thay máy cũ')
    expect(lastCall()).toEqual({ url: '/assets/a1/assign', method: 'POST', body: { assignee_id: 'u9', comment: 'Thay máy cũ' } })
    await assetApi.handOver('a1', 'u9')
    expect(lastCall().body).toEqual({ assignee_id: 'u9' })
  })

  it('a new asset sends type_id, name and custom_fields', async () => {
    await assetApi.create('ws1', { type_id: 't1', name: 'Laptop', custom_fields: { cfg: 'M3' } })
    expect(lastCall()).toEqual({ url: '/workspaces/ws1/assets', method: 'POST', body: { type_id: 't1', name: 'Laptop', custom_fields: { cfg: 'M3' } } })
  })
})

describe('asset type bodies', () => {
  it('a new type sends name and category', async () => {
    await assetApi.createType('ws1', { name: 'Laptop', category: 'hardware' })
    expect(lastCall()).toEqual({ url: '/workspaces/ws1/asset-types', method: 'POST', body: { name: 'Laptop', category: 'hardware' } })
  })

  it('saving fields sends the schema as a string, which is what the handler binds', async () => {
    await assetApi.updateTypeSchema('t1', { type: 'object', properties: { cfg: { title: 'Cấu hình', type: 'string', 'x-kind': 'text' } } })
    const { url, method, body } = lastCall()
    expect([url, method]).toEqual(['/asset-types/t1/schema', 'PUT'])
    expect(Object.keys(body)).toEqual(['fields_schema'])
    expect(typeof body.fields_schema).toBe('string')
    expect(JSON.parse(body.fields_schema).properties.cfg.title).toBe('Cấu hình')
  })
})
