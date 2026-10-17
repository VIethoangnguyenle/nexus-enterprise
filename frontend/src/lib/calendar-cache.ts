import type { EditScope, Occurrence, RsvpResponse } from '../api/calendar'

/** Identity of one instance: the event and which occurrence of it. */
export const instanceKey = (o: Pick<Occurrence, 'eventId' | 'occurrenceStart'>) => `${o.eventId}|${o.occurrenceStart}`

interface OccurrenceList {
  occurrences: Occurrence[]
}

/** A cached list with `fn` applied to the occurrences `hit` selects; the same list when nothing is hit. */
function mapList<L extends OccurrenceList>(list: L | undefined, hit: (o: Occurrence) => boolean, fn: (o: Occurrence) => Occurrence): L | undefined {
  if (!list || !list.occurrences.some(hit)) return list
  return { ...list, occurrences: list.occurrences.map((o) => (hit(o) ? fn(o) : o)) }
}

/** Moves one instance to new times (the optimistic half of a drag). Other instances wait for the server. */
export function moveInstance<L extends OccurrenceList>(
  list: L | undefined, target: Pick<Occurrence, 'eventId' | 'occurrenceStart'>, start: string, end: string,
): L | undefined {
  const key = instanceKey(target)
  return mapList(list, (o) => instanceKey(o) === key, (o) => ({ ...o, start, end }))
}

/** Removes the instances a delete covers: one, or the whole event for `all`. `following` is left to the server. */
export function removeInstances<L extends OccurrenceList>(
  list: L | undefined, target: Pick<Occurrence, 'eventId' | 'occurrenceStart'>, scope: EditScope | undefined,
): L | undefined {
  if (!list) return list
  const key = instanceKey(target)
  const gone = (o: Occurrence) => (scope === 'all' ? o.eventId === target.eventId : instanceKey(o) === key)
  if (!list.occurrences.some(gone)) return list
  return { ...list, occurrences: list.occurrences.filter((o) => !gone(o)) }
}

/** The caller's answer on one instance, or on every instance of the event for `all`. */
export function answerInstances<L extends OccurrenceList>(
  list: L | undefined, target: Pick<Occurrence, 'eventId' | 'occurrenceStart'>, response: RsvpResponse, scope: 'this' | 'all',
): L | undefined {
  const key = instanceKey(target)
  return mapList(
    list,
    (o) => (scope === 'all' ? o.eventId === target.eventId : instanceKey(o) === key),
    (o) => ({ ...o, myResponse: response }),
  )
}
