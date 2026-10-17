import { describe, expect, it } from 'vitest'
import type { ActivityEntry } from '../../api/assets'
import { recentAssetSteps, stepText } from './assets'

const step = (id: string, assetId: string, at: string, over: Partial<ActivityEntry> = {}): ActivityEntry => ({
  id, action: 'assign', from_state: 'available', to_state: 'assigned', asset_id: assetId, asset_name: `Tài sản ${assetId}`,
  actor_id: 'u1', actor_name: 'Trần Minh Đức', subject_user_id: 'u2', subject_name: 'Nguyễn Thu Lan', created_at: at, ...over,
})

describe('recentAssetSteps', () => {
  it('keeps the latest step of each asset, newest first', () => {
    const entries = [
      step('1', 'a', '2026-10-01T00:00:00Z'), step('2', 'b', '2026-10-03T00:00:00Z'), step('3', 'a', '2026-10-02T00:00:00Z'),
    ]
    expect(recentAssetSteps(entries).map((e) => e.id)).toEqual(['2', '3'])
  })

  it('leaves out decisions on requests, which are not an asset', () => {
    const entries = [step('r', '', '2026-10-09T00:00:00Z', { request_status: 'rejected', action: 'request_rejected' }), step('1', 'a', '2026-10-01T00:00:00Z')]
    expect(recentAssetSteps(entries).map((e) => e.id)).toEqual(['1'])
  })

  it('stops at the limit and suggests nothing from an empty feed', () => {
    const entries = Array.from({ length: 7 }, (_, i) => step(`${i}`, `x${i}`, `2026-10-0${i + 1}T00:00:00Z`))
    expect(recentAssetSteps(entries)).toHaveLength(5)
    expect(recentAssetSteps([])).toEqual([])
  })
})

describe('stepText', () => {
  it('says who did what, by name', () => {
    const text = stepText(step('1', 'a', '2026-10-01T00:00:00Z'))
    expect(text.startsWith('Trần Minh Đức ')).toBe(true)
    expect(text).toContain('Nguyễn Thu Lan')
    expect(text).not.toContain('Tài sản a')
  })
})
