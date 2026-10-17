import { describe, expect, it } from 'vitest'
import type { Occurrence } from '../api/calendar'
import { answerInstances, instanceKey, moveInstance, removeInstances } from './calendar-cache'

const occ = (eventId: string, occurrenceStart: string, over: Partial<Occurrence> = {}): Occurrence => ({
  eventId, occurrenceStart, start: occurrenceStart, end: occurrenceStart, allDay: false, calendarId: 'c', title: 't', busyOnly: false,
  isOrganizer: false, hasCall: false, isRecurring: true, canEdit: true, kind: 'event', guestCount: 0, version: 1, ...over,
})
const A1 = occ('a', '2026-10-12T09:00:00+07:00')
const A2 = occ('a', '2026-10-19T09:00:00+07:00')
const B1 = occ('b', '2026-10-12T09:00:00+07:00')
const list = { occurrences: [A1, A2, B1] }

describe('calendar cache patches', () => {
  it('moves exactly the one instance', () => {
    const next = moveInstance(list, A2, 'S', 'E')!
    expect(next.occurrences.map((o) => o.start)).toEqual([A1.start, 'S', B1.start])
    expect(next.occurrences[1]!.end).toBe('E')
  })

  it('gives back the same list when the instance is not in it', () => {
    expect(moveInstance(list, occ('zz', 'x'), 'S', 'E')).toBe(list)
    expect(moveInstance(undefined, A1, 'S', 'E')).toBeUndefined()
  })

  it('removes one instance, or the whole event for scope all', () => {
    expect(removeInstances(list, A1, 'this')!.occurrences).toEqual([A2, B1])
    expect(removeInstances(list, A1, undefined)!.occurrences).toEqual([A2, B1])
    expect(removeInstances(list, A1, 'all')!.occurrences).toEqual([B1])
  })

  it('answers one instance or all of the event', () => {
    expect(answerInstances(list, A1, 'declined', 'this')!.occurrences.map((o) => o.myResponse)).toEqual(['declined', undefined, undefined])
    expect(answerInstances(list, A1, 'accepted', 'all')!.occurrences.map((o) => o.myResponse)).toEqual(['accepted', 'accepted', undefined])
  })

  it('keys an instance by event and occurrence', () => {
    expect(instanceKey(A1)).not.toBe(instanceKey(A2))
    expect(instanceKey(A1)).not.toBe(instanceKey(B1))
  })
})
