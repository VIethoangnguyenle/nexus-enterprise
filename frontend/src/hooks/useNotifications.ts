import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { notificationApi } from '../api/notifications'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

export const notificationsQueryOptions = (limit = 10) =>
  queryOptions({ queryKey: keys.notifications.list(limit), queryFn: () => notificationApi.list(limit) })

export const unreadCountQueryOptions = () =>
  queryOptions({ queryKey: keys.notifications.unreadCount(), queryFn: () => notificationApi.unreadCount(), refetchInterval: 30_000 })

export function useNotifications(limit = 10) { return useQuery(notificationsQueryOptions(limit)) }
export function useUnreadCount() { return useQuery(unreadCountQueryOptions()) }

export function useMarkRead() {
  return useMutation({
    mutationFn: (id: string) => notificationApi.markRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: keys.notifications.all() })
    },
  })
}

export function useMarkAllRead() {
  return useMutation({
    mutationFn: () => notificationApi.markAllRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: keys.notifications.all() })
    },
  })
}
