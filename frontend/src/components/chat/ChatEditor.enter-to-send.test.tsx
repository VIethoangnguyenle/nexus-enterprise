import { cleanup, renderHook, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { settingsQueryKey } from '../../hooks/useServerSettings'
import { useEnterSends } from '../../hooks/useModuleSettings'
import { CH, MEMBERS, WS_ID } from '../../test/chat-fixtures'
import { useDraftsStore } from '../../stores/drafts.store'
import { ChatEditor } from './ChatEditor'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  // The settings are seeded in the cache; anything else asked for is not there.
  apiFetch: vi.fn(() => new Promise(() => {})),
}))

/** Below 1024px (`narrow`) or a wide screen; jsdom has no layout, so the media queries are answered by hand. */
function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

beforeEach(() => {
  localStorage.clear()
  useDraftsStore.getState().reset()
  queryClient.setQueryData(keys.messaging.members(CH.doisoat), { members: MEMBERS })
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
})

function open(enterSends: boolean) {
  const onSend = vi.fn()
  renderWithClient(<ChatEditor channelId={CH.doisoat} placeholder="Tin nhắn cho Đối soát" onSend={onSend} enterSends={enterSends} />)
  return { onSend, box: screen.getByRole('textbox', { name: 'Tin nhắn cho Đối soát' }) }
}

describe('ChatEditor: Enter để gửi off', () => {
  it('Enter makes a new line and sends nothing', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open(false)
    await user.click(box)
    await user.keyboard('một{Enter}hai')
    expect(onSend).not.toHaveBeenCalled()
    expect(box.querySelectorAll('p')).toHaveLength(2)
  })

  it('Shift+Enter sends, the other way round from the wide screen', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open(false)
    await user.click(box)
    await user.keyboard('xin chào{Shift>}{Enter}{/Shift}')
    expect(onSend).toHaveBeenCalledWith('<p>xin chào</p>', expect.any(Array))
  })

  it('Ctrl+Enter still sends', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open(false)
    await user.click(box)
    await user.keyboard('xin chào{Control>}{Enter}{/Control}')
    expect(onSend).toHaveBeenCalledWith('<p>xin chào</p>', expect.any(Array))
  })
})

describe('ChatEditor: Enter để gửi on', () => {
  it('Enter sends and Shift+Enter breaks the line', async () => {
    const user = userEvent.setup()
    const { onSend, box } = open(true)
    await user.click(box)
    await user.keyboard('một{Shift>}{Enter}{/Shift}hai')
    expect(onSend).not.toHaveBeenCalled()
    await user.keyboard('{Enter}')
    expect(onSend).toHaveBeenCalledTimes(1)
  })
})

describe('useEnterSends', () => {
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  const seed = (enter_to_send: boolean) => queryClient.setQueryData(settingsQueryKey('chat', WS_ID), { enter_to_send })

  it('below 1024px follows the setting, and changing it changes the keys', () => {
    viewport(true)
    seed(false)
    const { result, rerender } = renderHook(() => useEnterSends(WS_ID), { wrapper })
    expect(result.current).toBe(false)
    seed(true)
    rerender()
    expect(result.current).toBe(true)
  })

  it('below 1024px, before the settings answer, uses the default: Enter makes a new line', () => {
    viewport(true)
    const { result } = renderHook(() => useEnterSends(WS_ID), { wrapper })
    expect(result.current).toBe(false)
  })

  it('a wide screen always sends on Enter, whatever the setting', () => {
    viewport(false)
    seed(false)
    const { result } = renderHook(() => useEnterSends(WS_ID), { wrapper })
    expect(result.current).toBe(true)
  })
})
