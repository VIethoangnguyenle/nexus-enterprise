import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { keys } from '../../hooks/keys'
import { useQuoteWatch } from './use-quote-watch'

// The watch reads one list from the cache. Another component creating or
// refreshing an unrelated query (a call card mounting, say) must not re-render
// the composer: cache events arrive synchronously, during that component's render.
describe('useQuoteWatch', () => {
  it('re-renders only for its own list', async () => {
    const client = new QueryClient()
    client.setQueryData(keys.messaging.messages('ch-1'), { messages: [] })
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
    let renders = 0
    renderHook(() => { renders++; useQuoteWatch('channel:ch-1', 'ch-1', undefined) }, { wrapper })
    const before = renders

    act(() => { client.setQueryData(['calls', 'some-call'], { id: 'some-call' }) })
    expect(renders).toBe(before)

    // Its own list re-renders it — after the event, never inside it.
    act(() => { client.setQueryData(keys.messaging.messages('ch-1'), { messages: [{ id: 'm1' }] }) })
    await waitFor(() => expect(renders).toBe(before + 1))
  })
})
