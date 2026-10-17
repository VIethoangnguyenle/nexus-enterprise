import { cleanup, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouterProvider, createMemoryHistory, createRootRoute, createRouter } from '@tanstack/react-router'
import type { Message } from '../../api/messaging'
import { apiFetch } from '../../api/client'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, ME, CH, U } from '../../test/chat-fixtures'
import { E, calendarFixtureApi, enableGroupCalendar, resetFixtures } from '../../test/calendar-fixtures'
import { MessageBlock } from './MessageBlock'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

beforeEach(() => {
  resetFixtures()
  enableGroupCalendar()
  vi.mocked(apiFetch).mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  cleanup()
  resetClient()
})

const people = buildDirectory(CONTACTS)
const eventMessage: Message = {
  id: 'm-event', channel_id: CH.doisoat, sender_id: U.duc.id, sender_name: 'duc', message_type: 'system',
  content: '📅 Gặp đối tác VNPay', linked_entity_type: 'calendar_event', linked_entity_id: E.teamSync,
  created_at: '2026-10-11T03:31:00Z',
}

describe('MessageBlock with the event card', () => {
  it('shows the event under its creator and not the line written for clients without cards', async () => {
    const root = createRootRoute({
      component: () => <MessageBlock message={eventMessage} people={people} idBase="m-event" me={ME.id} />,
    })
    const router = createRouter({ routeTree: root, history: createMemoryHistory({ initialEntries: ['/'] }) })
    await router.load()
    renderWithClient(<RouterProvider router={router} />)
    expect(await screen.findByRole('group', { name: 'Sự kiện: Gặp đối tác VNPay' })).toBeInTheDocument()
    expect(screen.getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(screen.queryByText('📅 Gặp đối tác VNPay')).toBeNull()
    expectNoIds()
  })
})
