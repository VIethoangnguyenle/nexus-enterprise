import { describe, expect, it } from 'vitest'
import { arrange, type ArrangeInput } from './call-arrange'
import type { MediaTrack, ParticipantView } from './call-room'

const person = (id: string, over: Partial<ParticipantView> = {}): ParticipantView => ({
  id, name: id, isLocal: id === 'me', speaking: false, micOn: true, camOn: false, sharing: false,
  quality: 'good', handRank: null, handAt: null, camera: null, screen: null, ...over,
})
const room = (n: number) => ['me', ...Array.from({ length: n - 1 }, (_, i) => `u${i + 1}`)].map((id) => person(id))
const base = (n: number, over: Partial<ArrangeInput> = {}): ArrangeInput => ({
  participants: room(n), presenterId: null, speakerId: null, pinnedId: null, mode: 'auto', ...over,
})

describe('arrange', () => {
  it('shows a lone person large', () => {
    expect(arrange(base(1)).kind).toBe('single')
  })

  it('puts the other person large and you in the corner when two are in', () => {
    const a = arrange(base(2))
    expect(a).toMatchObject({ kind: 'pip', main: { id: 'u1' }, self: { id: 'me' } })
  })

  it.each([[3, 2], [4, 2], [5, 3], [9, 3], [10, 4], [20, 4]])('lays %i people on %i columns', (n, cols) => {
    expect(arrange(base(n))).toMatchObject({ kind: 'grid', cols })
  })

  it('pages the biggest room', () => {
    const a = arrange(base(20))
    expect(a.kind === 'grid' && a.tiles.length).toBe(20)
    expect(a.kind === 'grid' && a.perPage).toBeLessThan(20)
  })

  it('"Lưới" grids even two people', () => {
    expect(arrange(base(2, { mode: 'grid' }))).toMatchObject({ kind: 'grid', cols: 2 })
  })

  it('"Người đang nói" puts the speaker large with the rest in a strip', () => {
    const a = arrange(base(6, { mode: 'speaker', speakerId: 'u3' }))
    expect(a).toMatchObject({ kind: 'focus', show: 'camera', main: { id: 'u3' }, more: 1 })
    expect(a.kind === 'focus' && a.strip).toHaveLength(4)
  })

  it('a pinned tile wins over the layout choice', () => {
    expect(arrange(base(5, { mode: 'grid', pinnedId: 'u2' }))).toMatchObject({ kind: 'focus', main: { id: 'u2' } })
  })

  it('a presentation wins over everything and the strip keeps everyone, speakers first', () => {
    const screen = {} as MediaTrack
    const input = base(3, { pinnedId: 'u2', presenterId: 'u1' })
    input.participants[1] = person('u1', { sharing: true, screen })
    input.participants[2] = person('u2', { speaking: true })
    const a = arrange(input)
    expect(a).toMatchObject({ kind: 'focus', show: 'screen', main: { id: 'u1' } })
    expect(a.kind === 'focus' && a.strip.map((p) => p.id)).toEqual(['u2', 'me', 'u1'])
  })

  it('ignores a presenter whose picture has not arrived yet', () => {
    const input = base(2, { presenterId: 'u1' })
    expect(arrange(input).kind).toBe('pip')
  })
})
