import { MutationCache, QueryClient } from '@tanstack/react-query'
import { toast } from '../components/primitives/Toast'
import { explain, statusOf } from './errors'

declare module '@tanstack/react-query' {
  interface Register {
    mutationMeta: {
      /**
       * The caller reports this mutation's failure itself (its own toast or
       * inline message), so the shared handler must not add a second one.
       */
      silentError?: boolean
      /** What the user was doing, in the sentence "chưa <action> được": "xoá tệp". */
      action?: string
    }
  }
}

const DEFAULT_ACTION = 'thực hiện thao tác này'

/**
 * Every failed mutation tells the user something. A 401 is the exception: the
 * client has already refreshed once and, if that failed, ended the session, so
 * a toast on top of the redirect to sign-in would be noise.
 */
export const mutationCache = new MutationCache({
  onError: (error, _variables, _context, mutation) => {
    if (mutation.meta?.silentError) return
    if (statusOf(error) === 401) return
    toast.error(explain(error, mutation.meta?.action ?? DEFAULT_ACTION))
  },
})

export const queryClient = new QueryClient({
  mutationCache,
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 5 * 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
})
