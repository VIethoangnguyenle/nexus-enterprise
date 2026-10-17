import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Contact } from '../../hooks/useContacts'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import { ContactProfile } from './ContactProfile'

afterEach(() => {
  cleanup()
  useToastStore.getState().clear()
  vi.restoreAllMocks()
})

const lan: Contact = {
  user_id: '11111111-aaaa-4bbb-8ccc-000000000003', ngac_node_id: '22222222-aaaa-4bbb-8ccc-000000000003', username: 'lan',
  display_name: 'Nguyễn Thu Lan', email: 'lan@novapay.vn', title: 'Chuyên viên đối soát', department: 'Đối soát',
  department_id: '55555555-aaaa-4bbb-8ccc-000000000001', location: 'Hà Nội, tầng 12', avatar_url: '', is_online: false,
  roles: [
    { id: 'r-members', name: '', kind: 'members' },
    { id: 'r-pay', name: 'Duyệt chi', kind: 'custom' },
    { id: 'r-owners', name: '', kind: 'owners' },
  ],
}
const vinh: Contact = { ...lan, user_id: '11111111-aaaa-4bbb-8ccc-000000000004', display_name: 'Lê Quang Vinh', username: 'vinh' }
const ngoc: Contact = { ...lan, user_id: '11111111-aaaa-4bbb-8ccc-000000000006', display_name: 'Trần Bảo Ngọc', username: 'ngoc' }
const PATH = [{ id: 'k', name: 'Khối Vận hành' }, { id: lan.department_id, name: 'Đối soát' }]

function props(over: Partial<React.ComponentProps<typeof ContactProfile>> = {}) {
  return {
    contact: lan, online: false, isMe: false, isPersonal: false, path: PATH, colleagues: [vinh, ngoc], mode: 'panel' as const,
    busy: null, onMessage: vi.fn(), onCall: vi.fn(), onEdit: vi.fn(), onClose: vi.fn(), onOpenDept: vi.fn(), onOpenRole: vi.fn(),
    ...over,
  }
}

describe('ContactProfile: who', () => {
  it('shows the name as a heading, the title, and the status in words', () => {
    render(<ContactProfile {...props()} />)
    expect(screen.getByRole('heading', { name: 'Nguyễn Thu Lan' })).toBeInTheDocument()
    expect(screen.getByText('Chuyên viên đối soát')).toBeInTheDocument()
    expect(screen.getByText('Ngoại tuyến')).toBeInTheDocument()
  })

  it('says Trực tuyến when the person is online', () => {
    render(<ContactProfile {...props({ online: true })} />)
    expect(screen.getByText('Trực tuyến')).toBeInTheDocument()
  })

  it('puts the presence dot on the avatar of someone online, as the list does, and none on someone offline', () => {
    const { unmount } = render(<ContactProfile {...props({ online: true })} />)
    expect(document.querySelector('[title="Nguyễn Thu Lan"] .bg-success')).not.toBeNull()
    unmount()
    render(<ContactProfile {...props({ online: false })} />)
    expect(document.querySelector('[title="Nguyễn Thu Lan"] .bg-success')).toBeNull()
  })

  it('shows no identifier or username anywhere', () => {
    render(<ContactProfile {...props()} />)
    expectNoIds()
    expect(document.body.textContent).not.toContain('@lan')
  })
})

describe('ContactProfile: what you can do', () => {
  it('offers Nhắn tin, Gọi and Gửi email for someone else', async () => {
    const p = props()
    const user = userEvent.setup()
    render(<ContactProfile {...p} />)
    await user.click(screen.getByRole('button', { name: 'Nhắn tin' }))
    expect(p.onMessage).toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Gọi' }))
    expect(p.onCall).toHaveBeenCalled()
    expect(screen.getByRole('link', { name: 'Gửi email' })).toHaveAttribute('href', 'mailto:lan@novapay.vn')
  })

  it('writes the address into the mailto link so that no character of it ends the link', () => {
    render(<ContactProfile {...props({ contact: { ...lan, email: 'a+b&c?d@novapay.vn' } })} />)
    expect(screen.getByRole('link', { name: 'Gửi email' })).toHaveAttribute('href', 'mailto:a%2Bb%26c%3Fd@novapay.vn')
  })

  it('lets you call someone who is offline: the call rings their devices', () => {
    render(<ContactProfile {...props({ online: false })} />)
    expect(screen.getByRole('button', { name: 'Gọi' })).toBeEnabled()
  })

  it('waits while a message or a call is being started, so nothing starts twice', () => {
    render(<ContactProfile {...props({ busy: 'call' })} />)
    expect(screen.getByRole('button', { name: 'Gọi' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Nhắn tin' })).toBeDisabled()
  })

  it('offers only Sửa hồ sơ on one\'s own profile', async () => {
    const p = props({ isMe: true })
    render(<ContactProfile {...p} />)
    expect(screen.queryByRole('button', { name: 'Nhắn tin' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Gọi' })).toBeNull()
    expect(screen.queryByRole('link', { name: 'Gửi email' })).toBeNull()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Sửa hồ sơ' }))
    expect(p.onEdit).toHaveBeenCalled()
  })

  it('leaves out Gửi email for a person with no email', () => {
    render(<ContactProfile {...props({ contact: { ...lan, email: '' } })} />)
    expect(screen.queryByRole('link', { name: 'Gửi email' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Nhắn tin' })).toBeInTheDocument()
  })
})

describe('ContactProfile: details', () => {
  it('shows email with a copy button, the whole department path, roles by name, and where they work', () => {
    render(<ContactProfile {...props()} />)
    const dl = document.querySelector('dl') as HTMLElement
    expect(within(dl).getByText('lan@novapay.vn')).toBeInTheDocument()
    expect(within(dl).getByRole('button', { name: /Khối Vận hành/ })).toBeInTheDocument()
    expect(within(dl).getByRole('button', { name: /Đối soát/ })).toBeInTheDocument()
    // Members is held by everyone and never listed; the owners role reads Quản trị viên.
    expect(within(dl).getByRole('button', { name: 'Duyệt chi' })).toBeInTheDocument()
    expect(within(dl).getByRole('button', { name: 'Quản trị viên' })).toBeInTheDocument()
    expect(within(dl).getByText('Hà Nội, tầng 12')).toBeInTheDocument()
  })

  it('opens the department a path segment names, and the role a role names', async () => {
    const p = props()
    const user = userEvent.setup()
    render(<ContactProfile {...p} />)
    const dl = document.querySelector('dl') as HTMLElement
    await user.click(within(dl).getByRole('button', { name: /Khối Vận hành/ }))
    expect(p.onOpenDept).toHaveBeenLastCalledWith('k')
    await user.click(within(dl).getByRole('button', { name: /Đối soát/ }))
    expect(p.onOpenDept).toHaveBeenLastCalledWith(lan.department_id)
    await user.click(within(dl).getByRole('button', { name: 'Duyệt chi' }))
    expect(p.onOpenRole).toHaveBeenLastCalledWith('r-pay')
  })

  it('leaves out a detail that is empty, not a dash', () => {
    render(<ContactProfile {...props({ contact: { ...lan, location: '', roles: [] }, path: [] })} />)
    expect(screen.queryByText('Nơi làm việc')).toBeNull()
    expect(screen.queryByText('Vai trò')).toBeNull()
    expect(screen.queryByText('Phòng ban')).toBeNull()
    expect(screen.getByText('Email')).toBeInTheDocument()
  })

  it('copies the email and says so', async () => {
    const user = userEvent.setup() // installs its own clipboard; spy on that
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    render(<ContactProfile {...props()} />)
    await user.click(screen.getByRole('button', { name: 'Sao chép email' }))
    expect(writeText).toHaveBeenCalledWith('lan@novapay.vn')
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => t.message === 'Đã sao chép email')).toBe(true))
  })

  it('tells when the email could not be copied', async () => {
    const user = userEvent.setup()
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
    render(<ContactProfile {...props()} />)
    await user.click(screen.getByRole('button', { name: 'Sao chép email' }))
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => t.message === 'Không sao chép được email')).toBe(true))
  })
})

describe('ContactProfile: colleagues', () => {
  it('names the first and counts the others, and opens their department', async () => {
    const p = props()
    render(<ContactProfile {...p} />)
    const same = screen.getByRole('region', { name: 'Cùng phòng' })
    expect(within(same).getByText('Lê Quang Vinh và 1 người khác')).toBeInTheDocument()
    await userEvent.setup().click(within(same).getByRole('button'))
    expect(p.onOpenDept).toHaveBeenCalledWith(lan.department_id)
  })

  it('is left out when there is nobody else', () => {
    render(<ContactProfile {...props({ colleagues: [] })} />)
    expect(screen.queryByRole('region', { name: 'Cùng phòng' })).toBeNull()
  })
})

describe('ContactProfile: a personal workspace', () => {
  it('has no department, role or colleagues', () => {
    render(<ContactProfile {...props({ isPersonal: true })} />)
    expect(screen.queryByText('Phòng ban')).toBeNull()
    expect(screen.queryByText('Vai trò')).toBeNull()
    expect(screen.queryByRole('region', { name: 'Cùng phòng' })).toBeNull()
    expect(screen.getByText('Email')).toBeInTheDocument()
    expect(screen.getByText('Nơi làm việc')).toBeInTheDocument()
  })
})

describe('ContactProfile: focus', () => {
  it('moves focus to the name when a side panel opens with it, and not otherwise', () => {
    const { unmount } = render(<ContactProfile {...props({ focusHeading: true })} />)
    expect(screen.getByRole('heading', { name: 'Nguyễn Thu Lan' })).toHaveFocus()
    unmount()
    render(<ContactProfile {...props()} />)
    expect(screen.getByRole('heading', { name: 'Nguyễn Thu Lan' })).not.toHaveFocus()
  })
})

describe('ContactProfile: closing', () => {
  beforeEach(() => vi.clearAllMocks())

  it('closes with a button on the cover (panel and sheet)', async () => {
    const p = props()
    render(<ContactProfile {...p} />)
    await userEvent.setup().click(screen.getByRole('button', { name: 'Đóng hồ sơ' }))
    expect(p.onClose).toHaveBeenCalled()
  })

  it('on a phone the cover carries Quay lại instead, with a 44 hit area', async () => {
    const p = props({ mode: 'screen' })
    render(<ContactProfile {...p} />)
    expect(screen.queryByRole('button', { name: 'Đóng hồ sơ' })).toBeNull()
    const back = screen.getByRole('button', { name: 'Quay lại' })
    expect(back.className).toMatch(/\bw-11\b/)
    await userEvent.setup().click(back)
    expect(p.onClose).toHaveBeenCalled()
  })
})
