import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { authApi, type AccountLanguage, type MeResponse } from '../api/auth'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useMyProfile } from './useMyProfile'
import { keys } from './keys'

/**
 * The account's language (auth `account.language`, every workspace): what is chosen, Tiếng Việt when nothing is,
 * and a way to change it. The change shows at once and is put back, with the shared toast, when the server
 * refuses.
 */
export function useAccountLanguage() {
  const queryClient = useQueryClient()
  const { workspaceId } = useActiveWorkspace()
  const profile = useMyProfile(workspaceId)

  const mutation = useMutation({
    meta: { action: 'đổi ngôn ngữ' },
    mutationFn: (language: AccountLanguage) => authApi.updateProfile({ language }),
    onMutate: async (language) => {
      await queryClient.cancelQueries({ queryKey: keys.auth.profiles() })
      const before = queryClient.getQueriesData<MeResponse>({ queryKey: keys.auth.profiles() })
      queryClient.setQueriesData<MeResponse>({ queryKey: keys.auth.profiles() }, (old) =>
        old ? { ...old, user: { ...old.user, language } } : old)
      return { before }
    },
    onError: (_err, _language, context) => {
      for (const [key, data] of (context?.before ?? []) as [QueryKey, MeResponse | undefined][]) queryClient.setQueryData(key, data)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.auth.profiles() }),
  })

  const chosen = profile.data?.user.language
  return {
    value: (chosen === 'en' ? 'en' : 'vi') as AccountLanguage,
    ready: !!profile.data,
    set: (language: AccountLanguage) => mutation.mutate(language),
  }
}
