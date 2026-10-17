import { useMutation, useQuery } from '@tanstack/react-query'
import { calendarApi, type ShareLevel } from '../api/calendar'
import { toast } from '../components/primitives'
import { explain } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/** Who a calendar is shared with and how far; asked for only while the dialog is open. */
export function useShares(wsId: string, calId: string, enabled: boolean) {
  return useQuery({
    queryKey: keys.calendar.shares(wsId, calId),
    queryFn: () => calendarApi.listShares(wsId, calId),
    enabled: !!wsId && !!calId && enabled,
    select: (d) => d.shares ?? [],
  })
}

/** Shares with a person or changes their level; it takes effect for them at once. */
export function useSetShare(wsId: string, calId: string) {
  return useMutation({
    mutationFn: ({ nodeId, level }: { nodeId: string; level: ShareLevel; name: string; announce?: boolean }) =>
      calendarApi.setShare(wsId, calId, nodeId, level),
    onSuccess: (_share, { name, announce }) => {
      if (announce) toast(`Đã chia sẻ với ${name}`)
    },
    onError: (err) => toast.error(explain(err, 'chia sẻ lịch')),
    onSettled: () => void queryClient.invalidateQueries({ queryKey: keys.calendar.shares(wsId, calId) }),
  })
}

export function useRemoveShare(wsId: string, calId: string) {
  return useMutation({
    mutationFn: ({ nodeId }: { nodeId: string; name: string }) => calendarApi.removeShare(wsId, calId, nodeId),
    onSuccess: (_r, { name }) => toast(`Đã bỏ chia sẻ với ${name}`),
    onError: (err) => toast.error(explain(err, 'bỏ chia sẻ')),
    onSettled: () => void queryClient.invalidateQueries({ queryKey: keys.calendar.shares(wsId, calId) }),
  })
}

const refreshCalendars = (wsId: string) => {
  void queryClient.invalidateQueries({ queryKey: keys.calendar.calendars(wsId) })
  void queryClient.invalidateQueries({ queryKey: keys.calendar.eventsAll(wsId) })
}

/** Adds the personal calendar of another person to "Lịch khác". */
export function useSubscribe(wsId: string) {
  return useMutation({
    mutationFn: ({ nodeId }: { nodeId: string; name: string }) => calendarApi.subscribe(wsId, nodeId),
    onSuccess: (_c, { name }) => toast(`Đã thêm lịch của ${name}`),
    onError: (err) => toast.error(explain(err, 'thêm lịch')),
    onSettled: () => refreshCalendars(wsId),
  })
}

/** Takes a calendar out of the list (the calendar itself stays with its owner). */
export function useUnsubscribe(wsId: string) {
  return useMutation({
    mutationFn: ({ calId }: { calId: string; name: string }) => calendarApi.unsubscribe(wsId, calId),
    onSuccess: (_r, { name }) => toast(`Đã bỏ lịch ${name} khỏi danh sách`),
    onError: (err) => toast.error(explain(err, 'bỏ lịch')),
    onSettled: () => refreshCalendars(wsId),
  })
}
