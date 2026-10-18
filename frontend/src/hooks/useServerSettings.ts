import { useCallback, useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsApi, type SettingsByService, type SettingsService } from '../api/settings'
import { keys } from './keys'

/**
 * Where a service's settings are cached. Lịch's own settings dialog already reads
 * them under its key, so both screens show one value.
 */
export function settingsQueryKey(service: SettingsService, scopeId: string) {
  return service === 'calendar' ? keys.calendar.settings(scopeId) : keys.settings.scope(service, scopeId)
}

/**
 * A person's server-kept settings of one service in one scope (a workspace id).
 * Reading is a plain query; `save` shows the new value at once and puts the old
 * one back if the server refuses (the shared handler says so in a toast, "Chưa
 * lưu cài đặt được…").
 *
 * `defaults` are what the screen shows for a key the server has not answered.
 * A save sends only the change: every service's PUT keeps the keys a body leaves
 * out, so a stale copy of the others on this device never overwrites them.
 *
 * A failed read is not an error screen: the settings are then the defaults, as
 * for a key never set.
 */
export function useServerSettings<S extends SettingsService>(
  service: S,
  scopeId: string,
  defaults?: SettingsByService[S],
) {
  type Settings = SettingsByService[S]
  const queryClient = useQueryClient()
  const key = useMemo(() => settingsQueryKey(service, scopeId), [service, scopeId])

  const query = useQuery({
    queryKey: key,
    queryFn: () => settingsApi.get(service, scopeId),
    enabled: !!scopeId,
    // Defaults are an acceptable answer; do not hold the screen through retries.
    retry: false,
  })

  const mutation = useMutation({
    meta: { action: 'lưu cài đặt' },
    mutationFn: (body: Settings) => settingsApi.put(service, scopeId, body),
    onMutate: async (body) => {
      await queryClient.cancelQueries({ queryKey: key })
      const before = queryClient.getQueryData<Settings>(key)
      queryClient.setQueryData<Settings>(key, { ...before, ...body })
      return { before }
    },
    onError: (_error, _body, context) => {
      queryClient.setQueryData<Settings | undefined>(key, context?.before)
    },
    onSuccess: (saved) => {
      queryClient.setQueryData<Settings>(key, (old) => ({ ...old, ...saved }))
    },
  })

  const { mutateAsync } = mutation
  /** Resolves true once the server has it, false when it refused. */
  const save = useCallback(
    (patch: Settings) => mutateAsync(patch).then(() => true, () => false),
    [mutateAsync],
  )

  const settings = useMemo(() => ({ ...defaults, ...query.data }) as Settings, [defaults, query.data])

  return {
    settings,
    /** True until the first answer or failure; false when there is no scope to ask about. */
    isLoading: query.isLoading,
    isSaving: mutation.isPending,
    save,
  }
}
