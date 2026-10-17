import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Attendee } from '../../api/calendar'
import { buildDirectory } from '../../lib/people'
import type { EditorForm } from '../../lib/calendar-editor'
import { CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { DETAIL_ATTENDEES, PEOPLE, OCCURRENCES } from '../../test/calendar-fixtures'
import { renderWithClient } from '../../test/render'
import { EventDetailBody, EventPopover } from './EventPopover'
import { GuestsPanel } from './GuestsPanel'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

const state = vi.hoisted(() => ({ attendees: [] as unknown[], people: undefined as unknown }))
vi.mock('../../hooks/useCalendar', async (orig) => ({
  ...(await orig<typeof import('../../hooks/useCalendar')>()),
  useEventDetail: () => ({ data: { attendees: state.attendees }, isError: false, isPending: false }),
}))
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => state.people }))
vi.mock('../../hooks/useMessaging', async (orig) => ({
  ...(await orig<typeof import('../../hooks/useMessaging')>()),
  useChannels: () => ({ data: { channels: [] }, isLoading: false }),
}))

const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const HIDDEN_NODE = '22222222-aaaa-4bbb-8ccc-0000000000ff'
const RESOLVED = { userId: HIDDEN, nodeId: HIDDEN_NODE, name: 'Nguyễn Hữu Thành', avatarUrl: '' }
const SENTENCE = 'Hồ sơ này không còn hiển thị với bạn.'
const THANH = { nodeId: HIDDEN_NODE, displayName: 'Nguyễn Hữu Thành', hue: 4 }

beforeEach(() => {
  navigate.mockReset()
  state.people = {
    ...buildDirectory(CONTACTS, { resolved: { byUserId: new Map([[HIDDEN, RESOLVED]]), byNodeId: new Map([[HIDDEN_NODE, RESOLVED]]) } }),
    isLoading: false,
  }
})

async function expectCard() {
  const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
  expect(within(card).getByText(SENTENCE)).toBeInTheDocument()
  expect(within(card).queryAllByRole('button')).toHaveLength(0)
  expect(navigate).not.toHaveBeenCalled()
}

describe('EventPopover: pressing a guest', () => {
  const body = () => renderWithClient(
    <EventDetailBody
      wsId="w" occurrence={{ ...OCCURRENCES[0]!, guestCount: 3 }} onClose={() => {}} onDelete={() => {}} onRsvp={() => {}}
    />,
  )

  it('a hidden guest keeps the name from the calendar and opens the small card', async () => {
    state.attendees = [{ person: THANH, response: 'accepted', isOrganizer: false }] satisfies Attendee[]
    body()
    await userEvent.click(screen.getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    await expectCard()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('a visible guest opens their profile, the organiser included', async () => {
    state.attendees = DETAIL_ATTENDEES.map((a) => ({ ...a, person: { ...a.person, nodeId: { yen: U.yen.node, hoa: U.hoa.node, lan: U.lan.node }[a.person === PEOPLE.yen ? 'yen' : a.person === PEOPLE.hoa ? 'hoa' : 'lan'] } }))
    body()
    await userEvent.click(screen.getByRole('button', { name: 'Phạm Hải Yến' }))
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })
})

describe('EventPopover: the card inside the popover', () => {
  const open = () => {
    state.attendees = [{ person: THANH, response: 'accepted', isOrganizer: false }] satisfies Attendee[]
    const anchor = document.createElement('button')
    document.body.appendChild(anchor)
    renderWithClient(
      <EventPopover
        wsId="w" occurrence={{ ...OCCURRENCES[0]!, guestCount: 1 }} anchorRef={{ current: anchor }}
        onClose={() => {}} onDelete={() => {}} onRsvp={() => {}}
      />,
    )
  }

  it('a press inside the card does not close the event popover', async () => {
    open()
    await userEvent.click(await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    await userEvent.click(within(card).getByText(SENTENCE))
    expect(screen.getByRole('dialog', { name: 'Nguyễn Hữu Thành' })).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: OCCURRENCES[0]!.title })).toBeInTheDocument()
  })

  it('Esc closes only the card and puts focus back on the guest', async () => {
    open()
    const guest = await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' })
    await userEvent.click(guest)
    await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    await userEvent.keyboard('{Escape}')
    await vi.waitFor(() => expect(screen.queryByRole('dialog', { name: 'Nguyễn Hữu Thành' })).toBeNull())
    expect(screen.getByRole('dialog', { name: OCCURRENCES[0]!.title })).toBeInTheDocument()
    expect(guest).toHaveFocus()
  })
})

describe('GuestsPanel in the editor: names are plain', () => {
  const form = (guests: EditorForm['guests']) =>
    ({ guests, inviteChannelId: null, inviteChannelName: null, guestsCanInvite: false, guestsCanSeeList: true }) as unknown as EditorForm
  const panel = (guests: EditorForm['guests'], organizer?: EditorForm['guests'][number]) =>
    renderWithClient(<GuestsPanel wsId="w" form={form(guests)} onChange={() => {}} organizer={organizer} busy={new Map()} span={null} />)

  it('a guest name is not a button, so nothing can take the editor away from unsaved edits', async () => {
    panel([THANH, { nodeId: U.duc.node, displayName: 'Trần Minh Đức', hue: 1 }], { nodeId: U.hoa.node, displayName: 'Lê Thị Hoa', hue: 5 })
    for (const name of ['Nguyễn Hữu Thành', 'Trần Minh Đức', 'Lê Thị Hoa']) {
      expect(screen.getByText(name)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
    expect(screen.getByRole('button', { name: 'Bỏ Nguyễn Hữu Thành' })).toBeInTheDocument()
    await userEvent.click(screen.getByText('Lê Thị Hoa'))
    expect(navigate).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
