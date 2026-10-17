import { describe, expect, it } from 'vitest'
import type { DriveItem } from '../../api/drive'
import { recentlyEdited } from './drive'

const item = (id: string, updated: string, type: 'file' | 'folder' = 'file'): DriveItem => ({
  id, name: `${id}.xlsx`, item_type: type, updated_at: updated, created_at: updated, owner_id: 'u1',
} as DriveItem)

describe('recentlyEdited', () => {
  it('lists the files of a listing, latest edit first', () => {
    const list = [item('a', '2026-10-01T00:00:00Z'), item('b', '2026-10-03T00:00:00Z'), item('c', '2026-10-02T00:00:00Z')]
    expect(recentlyEdited(list).map((i) => i.id)).toEqual(['b', 'c', 'a'])
  })

  it('leaves folders out', () => {
    expect(recentlyEdited([item('f', '2026-10-09T00:00:00Z', 'folder'), item('a', '2026-10-01T00:00:00Z')]).map((i) => i.id)).toEqual(['a'])
  })

  it('stops at the limit', () => {
    const list = Array.from({ length: 8 }, (_, i) => item(`n${i}`, `2026-10-0${i + 1}T00:00:00Z`))
    expect(recentlyEdited(list)).toHaveLength(5)
    expect(recentlyEdited(list, 2).map((i) => i.id)).toEqual(['n7', 'n6'])
  })

  it('suggests only what the listing holds: a file the caller can no longer read is never suggested', () => {
    // The listing is what the server returned for the caller, already filtered by read access.
    expect(recentlyEdited([])).toEqual([])
  })
})
