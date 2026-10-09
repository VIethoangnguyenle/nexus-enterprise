import type { ReactElement, ReactNode } from 'react'
import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { ME } from './chat-fixtures'

function Providers({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}

/**
 * Renders with the app's own QueryClient (hooks invalidate through it
 * directly, so a separate test client would miss those writes) and a signed-in
 * user. The provider is a `wrapper`, so `rerender` keeps it. Call
 * `resetClient()` between tests.
 */
export function renderWithClient(ui: ReactElement) {
  useAuthStore.setState({ user: ME, accessToken: 'test-token', bootstrapping: false })
  // No retries: a fixture miss should fail at once, not after a backoff.
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: Infinity } })
  return render(ui, { wrapper: Providers })
}

export function resetClient() {
  queryClient.clear()
}
