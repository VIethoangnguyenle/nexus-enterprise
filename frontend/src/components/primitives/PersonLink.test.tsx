import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildDirectory, EMPTY_DIRECTORY } from '../../lib/people'
import { CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { PersonLink } from './PersonLink'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const people = buildDirectory(CONTACTS, {
  resolved: {
    byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Nguyễn Hữu Thành', avatarUrl: '' }]]),
    byNodeId: new Map(),
  },
})
const SENTENCE = 'Hồ sơ này không còn hiển thị với bạn.'

const phone = (on: boolean) => {
  window.matchMedia = ((query: string) => ({
    matches: on && query.includes('max-width: 767'), media: query, onchange: null,
    addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

beforeEach(() => { navigate.mockReset(); phone(false) })
afterEach(() => { delete (window as { matchMedia?: unknown }).matchMedia })

describe('PersonLink: a person the directory lists', () => {
  it('opens their profile through the callback', async () => {
    const onOpenProfile = vi.fn()
    render(<PersonLink userId={U.duc.id} people={people} onOpenProfile={onOpenProfile} />)
    await userEvent.click(screen.getByRole('button', { name: /Trần Minh Đức/ }))
    expect(onOpenProfile).toHaveBeenCalledWith(U.duc.id)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('by default goes to their profile in Danh bạ', async () => {
    render(<PersonLink userId={U.duc.id} people={people} />)
    await userEvent.click(screen.getByRole('button', { name: /Trần Minh Đức/ }))
    expect(navigate).toHaveBeenCalledTimes(1)
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })

  it('keeps the workspace the reader is in when it goes to Danh bạ', async () => {
    render(<PersonLink userId={U.duc.id} people={people} />)
    await userEvent.click(screen.getByRole('button', { name: /Trần Minh Đức/ }))
    const { search } = navigate.mock.calls[0]![0] as { search: (prev: { ws?: string }) => unknown }
    expect(typeof search).toBe('function')
    expect(search({ ws: 'w1' })).toEqual({ ws: 'w1', person: U.duc.id })
    expect(search({})).toEqual({ person: U.duc.id })
  })

  it('wraps whatever the caller shows, and still names the button', () => {
    render(<PersonLink userId={U.duc.id} people={people}><span>DT</span></PersonLink>)
    expect(screen.getByRole('button', { name: 'Trần Minh Đức' })).toHaveTextContent('DT')
  })
})

describe('PersonLink: a person the directory does not list', () => {
  it('shows the name, and a click opens the card with its one sentence instead of a profile', async () => {
    const onOpenProfile = vi.fn()
    render(<PersonLink userId={HIDDEN} people={people} onOpenProfile={onOpenProfile} />)
    await userEvent.click(screen.getByRole('button', { name: /Nguyễn Hữu Thành/ }))

    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(within(card).getByText(SENTENCE)).toBeInTheDocument()
    expect(onOpenProfile).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it('has no title, department, email, id or action in it', async () => {
    render(<PersonLink userId={HIDDEN} people={people} />)
    await userEvent.click(screen.getByRole('button'))
    const card = await screen.findByRole('dialog')
    expect(within(card).queryAllByRole('button')).toHaveLength(0)
    expect(within(card).queryAllByRole('link')).toHaveLength(0)
    expect(card.textContent).not.toMatch(/@|Nhắn tin|Gọi/)
    expect(card.textContent).not.toMatch(UUID_RE)
    expect(card.textContent).toContain('Nguyễn Hữu Thành')
    expect(card.textContent).not.toMatch(/Trưởng|Chuyên viên|phòng/i)
  })

  it('opens from the keyboard, closes on Esc and gives focus back', async () => {
    const user = userEvent.setup()
    render(<PersonLink userId={HIDDEN} people={people} />)
    const link = screen.getByRole('button', { name: /Nguyễn Hữu Thành/ })
    await user.tab()
    expect(link).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await vi.waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(link).toHaveFocus()
  })

  it('closes on a press outside', async () => {
    render(<div><PersonLink userId={HIDDEN} people={people} /><p>Ngoài</p></div>)
    await userEvent.click(screen.getByRole('button'))
    await screen.findByRole('dialog')
    await userEvent.click(screen.getByText('Ngoài'))
    await vi.waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('is the same card for someone nobody has named yet, called by the neutral word', async () => {
    render(<PersonLink userId="11111111-aaaa-4bbb-8ccc-000000000abc" people={people} />)
    expect(screen.getByRole('button')).toHaveTextContent('Thành viên')
    await userEvent.click(screen.getByRole('button'))
    expect(within(await screen.findByRole('dialog')).getByText(SENTENCE)).toBeInTheDocument()
  })

  it('is a bottom sheet on a phone, closed by Esc', async () => {
    phone(true)
    const user = userEvent.setup()
    render(<PersonLink userId={HIDDEN} people={people} />)
    await user.click(screen.getByRole('button'))
    const sheet = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    expect(within(sheet).getByText(SENTENCE)).toBeInTheDocument()
    await act(async () => { await user.keyboard('{Escape}') })
    await vi.waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})

describe('PersonLink: before the directory is known', () => {
  it('does nothing on a press, claims no card and says it is busy', async () => {
    const onOpenProfile = vi.fn()
    render(<PersonLink userId={U.duc.id} people={EMPTY_DIRECTORY} onOpenProfile={onOpenProfile} />)
    const link = screen.getByRole('button')
    expect(link).toHaveAttribute('aria-busy', 'true')
    expect(link).not.toHaveAttribute('aria-haspopup')
    expect(link).not.toHaveAttribute('aria-expanded')
    await userEvent.click(link)
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onOpenProfile).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it('becomes a profile link once the directory lists them, and a card-opener only for someone it does not', async () => {
    const onOpenProfile = vi.fn()
    const { rerender } = render(<PersonLink userId={U.duc.id} people={EMPTY_DIRECTORY} onOpenProfile={onOpenProfile} />)
    rerender(<PersonLink userId={U.duc.id} people={people} onOpenProfile={onOpenProfile} />)
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-busy')
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-haspopup')
    await userEvent.click(screen.getByRole('button'))
    expect(onOpenProfile).toHaveBeenCalledWith(U.duc.id)

    rerender(<PersonLink userId={HIDDEN} people={people} onOpenProfile={onOpenProfile} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-haspopup', 'dialog')
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'false')
  })
})
