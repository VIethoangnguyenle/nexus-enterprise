import { useMemo } from 'react'
import { useSubscribe } from '../../hooks/useCalendarSharing'
import { usePeople } from '../../hooks/usePeople'
import type { CalendarSummary } from '../../api/calendar'
import { useAuthStore } from '../../stores/auth.store'
import { PeoplePicker } from '../primitives'

/**
 * Tìm người (DESIGN.md §6, Cột lịch): pick a person and their personal calendar joins "Lịch khác". People whose
 * calendar is already listed, and the person themself, are not offered. Nothing here takes an id by hand.
 */
export function AddCalendarPicker({ wsId, calendars }: { wsId: string; calendars: CalendarSummary[] }) {
  const people = usePeople(wsId)
  const subscribe = useSubscribe(wsId)
  const me = useAuthStore((s) => s.user?.id)
  const exclude = useMemo(() => {
    const out = new Set<string>(me ? [me] : [])
    for (const c of calendars) {
      const p = c.owner && people.byNodeId.get(c.owner.nodeId)
      if (p) out.add(p.userId)
    }
    return out
  }, [calendars, people.byNodeId, me])
  return (
    <PeoplePicker
      label="Tìm người"
      placeholder="Tìm người"
      people={people.list}
      value={[]}
      exclude={exclude}
      onChange={(next) => {
        const p = next[0]
        if (p) subscribe.mutate({ nodeId: p.nodeId, name: p.name })
      }}
    />
  )
}
