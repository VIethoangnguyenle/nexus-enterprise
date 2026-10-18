import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { authApi } from '../api/auth'
import { keys } from './keys'

/** The signed-in person's own sessions (Thiết bị và phiên): the one in use first. */
export function useSessions() {
  return useQuery({
    queryKey: keys.auth.sessions(),
    queryFn: () => authApi.listSessions().then((r) => r.sessions ?? []),
  })
}

/** Ends one of the person's other sessions; the list drops it once the server agrees. */
export function useRevokeSession() {
  const queryClient = useQueryClient()
  return useMutation({
    meta: { action: 'đăng xuất thiết bị này' },
    mutationFn: (sessionId: string) => authApi.revokeSession(sessionId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.auth.sessions() }),
  })
}
