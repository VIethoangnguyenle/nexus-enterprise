import { cleanup, configure, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CAL, calendarFixtureApi, calls, enableGroupCalendar, mode, resetFixtures } from '../../test/calendar-fixtures'
import { CH } from '../../test/chat-fixtures'
import { validateNewEventSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { ComposerCreate } from './ComposerCreate'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

configure({ asyncUtilTimeout: 4000 })

async function renderCreate(onClose: () => void) {
  const root = createRootRoute()
  const here = createRoute({ getParentRoute: () => root, path: '/', component: () => <ComposerCreate kind="event" channelId={CH.doisoat} onClose={onClose} /> })
  const added = createRoute({ getParentRoute: () => root, path: '/calendar/new', validateSearch: validateNewEventSearch, component: () => <p>Trình soạn</p> })
  const router = createRouter({ routeTree: root.addChildren([here, added]), history: createMemoryHistory({ initialEntries: ['/'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  resetFixtures()
  vi.mocked(apiFetch).mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  cleanup()
  resetClient()
  useToastStore.getState().clear()
})

describe('Sự kiện from the "+" menu', () => {
  it('opens the editor on the calendar of the group, making it the first time, and lets the menu go', async () => {
    const onClose = vi.fn()
    const router = await renderCreate(onClose)
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ calendar: CAL.group })
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(1)
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('uses the calendar the group already has', async () => {
    enableGroupCalendar()
    const router = await renderCreate(vi.fn())
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  it('tells the person and stays in the chat when the group refuses', async () => {
    mode.groupForbidden = true
    const onClose = vi.fn()
    const router = await renderCreate(onClose)
    await waitFor(() => expect(useToastStore.getState().toasts.length).toBeGreaterThan(0))
    expect(router.state.location.pathname).toBe('/')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })
})
