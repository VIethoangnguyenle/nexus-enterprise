import type { ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, screen } from '@testing-library/react'
import { renderWithClient, resetClient } from '../test/render'
import { Route } from './_workspace'

let channels: Array<{ channel_id: string; unread_count: number; last_read_message_id: string; notification_level?: 'all' | 'main' | 'off'; mention_count?: number }> = []

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router')
  return { ...actual, Outlet: () => null, Navigate: () => null, useMatches: () => [{ pathname: '/channels' }] }
})
vi.mock('../hooks/useActiveWorkspace', () => ({
  useActiveWorkspace: () => ({ workspaceId: 'w1', workspaceName: 'Khối Vận hành', workspaces: [{ id: 'w1' }], isLoading: false, isError: false }),
}))
vi.mock('../hooks/usePreferences', () => ({ usePreferencesSync: () => {} }))
vi.mock('../hooks/useWorkspaceRealtime', () => ({ useWorkspaceRealtime: () => {} }))
vi.mock('../hooks/useMessaging', () => ({ useUnreadCounts: () => ({ data: { channels } }) }))
vi.mock('../stores/websocket.store', () => ({
  useWebSocketStore: (select: (s: { connect: () => void; disconnect: () => void }) => unknown) => select({ connect: () => {}, disconnect: () => {} }),
}))
vi.mock('../components/patterns/AppSidebar', () => ({
  AppSidebar: ({ unreadCounts }: { unreadCounts: { messaging?: number } }) => <div data-testid="sidebar" data-unread={unreadCounts.messaging} />,
}))
vi.mock('../components/patterns/ListPanel', () => ({ ListPanel: () => null }))
vi.mock('../components/patterns/MobileNav', () => ({ MobileNav: () => null }))
vi.mock('../components/patterns/PhoneSearchBar', () => ({ PhoneSearchBar: () => null }))
vi.mock('../components/app/OfflineBar', () => ({ OfflineBar: () => null }))
vi.mock('../components/app/UpdateBar', () => ({ UpdateBar: () => null }))

afterEach(() => {
  cleanup()
  resetClient()
})

const row = (id: string, unread_count: number, over: Partial<(typeof channels)[number]> = {}) =>
  ({ channel_id: id, unread_count, last_read_message_id: '', ...over })

const badge = () => {
  const Layout = Route.options.component as () => ReactElement
  renderWithClient(<Layout />)
  return screen.getByTestId('sidebar').getAttribute('data-unread')
}

describe('workspace layout: sidebar Tin nhắn badge', () => {
  it('sums unread by notification level; a muted conversation adds only the times you were named', () => {
    channels = [row('c1', 3), row('c2', 5, { notification_level: 'off', mention_count: 1 }), row('c3', 7, { notification_level: 'off' })]
    expect(badge()).toBe('4')
  })

  it('is zero when only muted conversations without a mention are unread', () => {
    channels = [row('c1', 9, { notification_level: 'off' })]
    expect(badge()).toBe('0')
  })
})

describe('workspace layout: room under the list for the phone Chat bar', () => {
  const stubViewport = (narrow: boolean) =>
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: narrow && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
  const main = () => {
    const Layout = Route.options.component as () => ReactElement
    const { container } = renderWithClient(<Layout />)
    return container.querySelector('main')!
  }
  afterEach(() => vi.unstubAllGlobals())

  it('keeps 108px and the safe-area inset under the list of conversations on a phone', () => {
    stubViewport(true)
    channels = []
    expect(main().className).toContain('pb-[calc(6.75rem+env(safe-area-inset-bottom,0px))]')
  })

  it('keeps nothing at desktop widths', () => {
    stubViewport(false)
    channels = []
    expect(main().className).not.toContain('pb-[calc(6.75rem')
  })
})
