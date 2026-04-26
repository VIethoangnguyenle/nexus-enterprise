import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { notificationApi } from '../api/notifications'
import { queryClient } from '../lib/query-client'

export const notificationsQueryOptions = (limit = 10) =>
  queryOptions({ queryKey: ['notifications', limit], queryFn: () => notificationApi.list(limit) })

export const unreadCountQueryOptions = () =>
  queryOptions({ queryKey: ['unread-count'], queryFn: () => notificationApi.unreadCount(), refetchInterval: 30_000 })

export function useNotifications(limit = 10) { return useQuery(notificationsQueryOptions(limit)) }
export function useUnreadCount() { return useQuery(unreadCountQueryOptions()) }

export function useMarkRead() {
  return useMutation({
    mutationFn: (id: string) => notificationApi.markRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
    },
  })
}

export function useMarkAllRead() {
  return useMutation({
    mutationFn: () => notificationApi.markAllRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
    },
  })
}
