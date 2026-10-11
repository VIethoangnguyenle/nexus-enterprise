import { act, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { renderWithClient } from '../test/render'
import { useBackOrHome } from './useShortcutScreen'

function mount(entries: string[]) {
  let back!: () => void
  const root = createRootRoute({
    component: function Host() {
      back = useBackOrHome()
      return <p>trang</p>
    },
  })
  const pages = ['/channels', '/channels/starred'].map((path) => createRoute({ getParentRoute: () => root, path, component: () => null }))
  const history = createMemoryHistory({ initialEntries: entries, initialIndex: entries.length - 1 })
  const router = createRouter({ routeTree: root.addChildren(pages), history })
  renderWithClient(<RouterProvider router={router} />)
  return { router, back: () => back() }
}

describe('Quay lại', () => {
  it('goes back inside the app when the screen was reached from another one', async () => {
    const { router, back } = mount(['/channels', '/channels/starred'])
    await screen.findByText('trang')
    act(() => back())
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
  })

  it('goes to Trang chủ, and does not leave the app, when the screen was the first one opened', async () => {
    const leave = vi.spyOn(window.history, 'back')
    // The browser's own history is long (other sites came before), the app's is one entry.
    vi.spyOn(window.history, 'length', 'get').mockReturnValue(5)
    const { router, back } = mount(['/channels/starred'])
    await screen.findByText('trang')
    act(() => back())
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    expect(leave).not.toHaveBeenCalled()
  })
})
