import { useCallback } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { pushApi, pushQueryKeys, type PushPreferences } from '../lib/push'
import { toast } from '../components/primitives'

/**
 * The push preferences as messaging keeps them (`/api/me/push-preferences`):
 * the kinds, quiet hours and the pause. `pause_until` is an RFC 3339 moment
 * while paused; null or absent is not paused. A PUT that leaves it out keeps
 * the pause as it is; null resumes.
 */
export interface PushPreferencesState extends PushPreferences {
  pause_until?: string | null
}

export const browserZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Ho_Chi_Minh'

/** What a person who never saved anything has: every kind on, no quiet hours, not paused. */
export const DEFAULT_PUSH_PREFERENCES: PushPreferencesState = {
  kinds: { direct: true, mention: true, thread_reply: true, incoming_call: true },
  quiet_hours: { enabled: false, start: '22:00', end: '07:00', time_zone: browserZone() },
}

/** True while `pause_until` is a moment still ahead of `now`. */
export function isPaused(prefs: Pick<PushPreferencesState, 'pause_until'> | undefined, now = Date.now()): boolean {
  const until = prefs?.pause_until ? Date.parse(prefs.pause_until) : NaN
  return Number.isFinite(until) && until > now
}

/**
 * The signed-in person's push preferences, read once and shared by every screen
 * that shows them (Cài đặt → Thông báo, Tôi → Tạm dừng thông báo, Cài đặt Chat).
 * `save` sends the whole value with the change applied (the server replaces the
 * kinds and quiet hours), shows it at once, and puts the old value back with a
 * toast when the server refuses. A screen that changes the quiet hours sends
 * this browser's zone with them (`browserZone`), so the window is where the
 * person is.
 */
export function usePushPreferences(enabled = true) {
  const queryClient = useQueryClient()
  const key = pushQueryKeys.preferences

  const query = useQuery({
    queryKey: key,
    queryFn: () => pushApi.getPreferences() as Promise<PushPreferencesState>,
    enabled,
  })

  const mutation = useMutation({
    meta: { silentError: true },
    mutationFn: (next: PushPreferencesState) => pushApi.updatePreferences(next) as Promise<PushPreferencesState>,
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<PushPreferencesState>(key)
      queryClient.setQueryData<PushPreferencesState>(key, next)
      return { previous }
    },
    onError: (_err, _next, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous)
      toast.error('Không lưu được cài đặt thông báo')
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: key })
    },
  })

  const { mutateAsync } = mutation
  /** Applies `change` to what is shown now. Resolves true once saved, false when refused. */
  const save = useCallback(
    (change: (current: PushPreferencesState) => PushPreferencesState) => {
      const current = queryClient.getQueryData<PushPreferencesState>(key) ?? DEFAULT_PUSH_PREFERENCES
      return mutateAsync(change(current)).then(() => true, () => false)
    },
    [mutateAsync, queryClient, key],
  )

  return {
    /** The stored preferences, the defaults while loading; undefined after a failed read. */
    prefs: query.data ?? (query.isError ? undefined : DEFAULT_PUSH_PREFERENCES),
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
    isSaving: mutation.isPending,
    save,
  }
}
