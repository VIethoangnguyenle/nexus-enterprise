import { describe, it, expect, vi, beforeEach } from 'vitest'

const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...a: unknown[]) => apiFetch(...a) }))

const { NGAC_OPS, batchCheckAccess } = await import('./access')

beforeEach(() => {
  apiFetch.mockReset().mockResolvedValue({ results: {} })
})

describe('NGAC operations', () => {
  it('are exactly the eight the policy service evaluates', () => {
    expect([...NGAC_OPS].sort()).toEqual(
      ['approve', 'create_channel', 'invite', 'manage', 'read', 'share', 'upload', 'write'],
    )
  })

  it('are all requested by default, and never the non-operation "delete"', async () => {
    await batchCheckAccess(['node-1'])
    const body = JSON.parse(apiFetch.mock.calls[0]![1].body as string)
    expect(body.object_ids).toEqual(['node-1'])
    expect([...body.operations].sort()).toEqual([...NGAC_OPS].sort())
    expect(body.operations).not.toContain('delete')
  })
})
