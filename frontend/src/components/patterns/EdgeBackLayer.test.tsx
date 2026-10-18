import { act, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter, useMatches,
} from '@tanstack/react-router'
import { renderWithClient } from '../../test/render'
import { touch, touchSwipe } from '../../test/gesture-events'
import { PhoneEdgeBack } from './EdgeBackLayer'

let phone = true
let standalone = true
vi.mock('../../hooks/usePhone', () => ({ useBottomBarLayout: () => phone, usePhone: () => phone }))
vi.mock('../../lib/push', () => ({ isStandalone: () => standalone }))

function mount(entries: string[]) {
  const root = createRootRoute({
    component: function Shell() {
      const matches = useMatches()
      const path = matches[matches.length - 1]?.pathname ?? ''
      return (
        <PhoneEdgeBack pathname={path}>
          <Outlet />
        </PhoneEdgeBack>
      )
    },
  })
  const list = createRoute({ getParentRoute: () => root, path: '/channels', component: () => <p>Danh sách Chat</p> })
  const conversation = createRoute({ getParentRoute: () => root, path: '/channels/$id', component: () => <p>Cuộc trò chuyện</p> })
  const history = createMemoryHistory({ initialEntries: entries, initialIndex: entries.length - 1 })
  const router = createRouter({ routeTree: root.addChildren([list, conversation]), history })
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const screenEl = () => screen.getByTestId('edge-back-screen')

beforeEach(() => {
  phone = true
  standalone = true
  Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true, writable: true })
})

describe('PhoneEdgeBack', () => {
  it('goes back to the screen before, which peeks underneath while the finger drags', async () => {
    const router = mount(['/channels'])
    await screen.findByText('Danh sách Chat')
    // A step of the app (not a page load), so the router announces it and the list is kept for the peek.
    act(() => router.history.push('/channels/c1'))
    await screen.findByText('Cuộc trò chuyện')

    const el = screenEl()
    touch(el, 'touchstart', { x: 5, t: 0 })
    touch(el, 'touchmove', { x: 20, t: 16 })
    touch(el, 'touchmove', { x: 220, t: 800 })
    const peek = await screen.findByTestId('edge-back-peek')
    // A still, unreachable copy of the list that was left.
    expect(peek).toHaveTextContent('Danh sách Chat')
    expect(peek).toHaveAttribute('aria-hidden', 'true')
    touch(el, 'touchend', { x: 220, t: 810 })

    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    await waitFor(() => expect(screen.queryByTestId('edge-back-peek')).toBeNull())
    expect(screen.getAllByText('Danh sách Chat')).toHaveLength(1)
  })

  it('opened from a push notification, with nothing before it, lands on Chat’s root list', async () => {
    const router = mount(['/channels/c1'])
    await screen.findByText('Cuộc trò chuyện')
    touchSwipe(screenEl(), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 260, t: 800 }])
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    // Replaced, not pushed: back from the list does not return to the conversation.
    expect(router.history.length).toBe(1)
  })

  it('does nothing on a list screen with the bottom bar', async () => {
    const router = mount(['/channels/c1', '/channels'])
    await screen.findByText('Danh sách Chat')
    touchSwipe(screenEl(), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 300, t: 800 }])
    await new Promise((r) => setTimeout(r, 350))
    expect(router.state.location.pathname).toBe('/channels')
  })

  it('in a browser tab (not the installed app) leaves the edge to the browser and the ← button', async () => {
    standalone = false
    const router = mount(['/channels', '/channels/c1'])
    await screen.findByText('Cuộc trò chuyện')
    expect(screen.queryByTestId('edge-back-screen')).toBeNull()
    touchSwipe(screen.getByText('Cuộc trò chuyện'), [{ x: 5, t: 0 }, { x: 20, t: 16 }, { x: 300, t: 800 }])
    await new Promise((r) => setTimeout(r, 350))
    expect(router.state.location.pathname).toBe('/channels/c1')
  })

  it('renders the screen with nothing around it on a desktop', async () => {
    phone = false
    mount(['/channels/c1'])
    await screen.findByText('Cuộc trò chuyện')
    expect(screen.queryByTestId('edge-back-screen')).toBeNull()
  })
})
