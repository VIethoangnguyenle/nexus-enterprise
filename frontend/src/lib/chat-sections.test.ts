import { describe, expect, it } from 'vitest'
import type { Conversation } from './conversations'
import {
  arrange, sectionOfChannel, validateSectionName,
  withMoved, withPlaced, withRenamed, withRestored, withUnplaced, withoutSection,
  type ChatSection,
} from './chat-sections'

const conv = (id: string, kind: 'dm' | 'space', unread = 0): Conversation => ({
  id, kind, title: id, hueKey: id, unread, memberCount: 0, lastActivity: 0,
})

const sec = (id: string, name: string, position: number, ids: string[] = []): ChatSection => ({
  id, name, position, channel_ids: ids,
})

describe('arrange', () => {
  const all = [conv('a', 'space', 2), conv('b', 'dm', 1), conv('c', 'space'), conv('d', 'dm')]

  it('keeps default lists when there are no sections', () => {
    const r = arrange(all, [])
    expect(r.sections).toEqual([])
    expect(r.spaces.map((c) => c.id)).toEqual(['a', 'c'])
    expect(r.directs.map((c) => c.id)).toEqual(['b', 'd'])
  })

  it('takes a placed conversation out of its default list and keeps the conversation order inside the section', () => {
    const r = arrange(all, [sec('s1', 'Q4', 0, ['d', 'a'])])
    expect(r.sections[0]!.items.map((c) => c.id)).toEqual(['a', 'd'])
    expect(r.spaces.map((c) => c.id)).toEqual(['c'])
    expect(r.directs.map((c) => c.id)).toEqual(['b'])
  })

  it('totals unread per section for the collapsed header', () => {
    const r = arrange(all, [sec('s1', 'Q4', 0, ['a', 'b'])])
    expect(r.sections[0]!.unread).toBe(3)
  })

  it('orders sections by position whatever order they arrive in', () => {
    const r = arrange(all, [sec('s2', 'B', 5), sec('s1', 'A', 1)])
    expect(r.sections.map((s) => s.id)).toEqual(['s1', 's2'])
  })

  it('ignores a channel the user cannot list, and lists a conversation once if two sections claim it', () => {
    const r = arrange(all, [sec('s1', 'A', 0, ['ghost', 'a']), sec('s2', 'B', 1, ['a'])])
    expect(r.sections[0]!.items.map((c) => c.id)).toEqual(['a'])
    expect(r.sections[1]!.items).toEqual([])
  })
})

describe('validateSectionName', () => {
  const existing = [sec('s1', 'Quyết toán quý 4', 0), sec('s2', 'Nội bộ', 1)]

  it('asks for a name when blank', () => {
    expect(validateSectionName('   ', existing)).toBe('Nhập tên mục')
  })

  it('rejects a duplicate whatever its case, but lets a section keep its own name', () => {
    expect(validateSectionName('  nội BỘ ', existing)).toBe('Bạn đã có mục tên này')
    expect(validateSectionName('NỘI BỘ', existing, 's2')).toBeNull()
  })

  it('rejects more than 40 characters after trimming', () => {
    expect(validateSectionName('x'.repeat(41), existing)).toMatch(/40/)
    expect(validateSectionName(` ${'x'.repeat(40)} `, existing)).toBeNull()
  })
})

describe('optimistic reducers', () => {
  const base = [sec('s1', 'A', 0, ['a']), sec('s2', 'B', 1, ['b']), sec('s3', 'C', 2)]

  it('renames', () => {
    expect(withRenamed(base, 's2', 'Bb').map((s) => s.name)).toEqual(['A', 'Bb', 'C'])
  })

  it('moves a section up or down by swapping with its neighbour, and not past either end', () => {
    expect(withMoved(base, 's2', 'up').map((s) => s.id)).toEqual(['s2', 's1', 's3'])
    expect(withMoved(base, 's2', 'down').map((s) => s.id)).toEqual(['s1', 's3', 's2'])
    expect(withMoved(base, 's1', 'up')).toEqual(base)
    expect(withMoved(base, 's3', 'down')).toEqual(base)
    expect(withMoved(base, 's2', 'up').map((s) => s.position)).toEqual([0, 1, 2])
  })

  it('places a conversation in one section only, moving it from another', () => {
    const r = withPlaced(base, 's2', 'a')
    expect(r.find((s) => s.id === 's1')!.channel_ids).toEqual([])
    expect(r.find((s) => s.id === 's2')!.channel_ids).toEqual(['b', 'a'])
  })

  it('unplaces', () => {
    expect(withUnplaced(base, 'a').find((s) => s.id === 's1')!.channel_ids).toEqual([])
  })

  it('removes a section and restores it at its position with its conversations', () => {
    const removed = withoutSection(base, 's2')
    expect(removed.map((s) => s.id)).toEqual(['s1', 's3'])
    const back = withRestored(removed, sec('s2', 'B', 1, ['b']))
    expect(back.map((s) => s.id)).toEqual(['s1', 's2', 's3'])
    expect(back[1]!.channel_ids).toEqual(['b'])
  })

  it('a restored section takes its conversations back from wherever they went meanwhile', () => {
    const back = withRestored(withPlaced(withoutSection(base, 's2'), 's1', 'b'), sec('s2', 'B', 1, ['b']))
    expect(back.find((s) => s.id === 's1')!.channel_ids).toEqual(['a'])
  })
})

describe('sectionOfChannel', () => {
  it('finds the section a conversation sits in', () => {
    expect(sectionOfChannel([sec('s1', 'A', 0, ['a'])], 'a')?.id).toBe('s1')
    expect(sectionOfChannel([sec('s1', 'A', 0, ['a'])], 'z')).toBeUndefined()
  })
})
