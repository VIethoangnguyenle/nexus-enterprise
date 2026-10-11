import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Conversation } from '../../lib/conversations'
import { ConversationTitleMenu } from './ConversationTitleMenu'

const space: Conversation = { id: 'c1', kind: 'space', title: 'Đối soát giao dịch', hueKey: 'c1', unread: 0, memberCount: 14, lastActivity: 0 }
const dm: Conversation = { id: 'c2', kind: 'dm', title: 'Phạm Hải Yến', hueKey: 'c2', unread: 0, memberCount: 2, lastActivity: 0 }

const mount = (over: Partial<Parameters<typeof ConversationTitleMenu>[0]> = {}) => {
  const props = {
    conversation: space, level: 'main' as const, starred: false,
    onChooseLevel: vi.fn(), onToggleStar: vi.fn(), onOpenInfo: vi.fn(), ...over,
  }
  render(<ConversationTitleMenu {...props} />)
  return props
}
const trigger = () => screen.getByRole('button', { name: /Đối soát giao dịch|Phạm Hải Yến/ })

describe('ConversationTitleMenu', () => {
  it('is the conversation name as a heading holding the button that opens the menu', () => {
    mount()
    expect(screen.getByRole('heading', { name: 'Đối soát giao dịch' })).toContainElement(trigger())
    expect(trigger()).toHaveAttribute('aria-haspopup', 'menu')
    expect(trigger()).toHaveAttribute('aria-expanded', 'false')
    expect(trigger().className).toContain('min-h-9')
  })

  it('opens a menu led by the Thông báo group: three radios, the current one checked, the default tagged', async () => {
    const user = userEvent.setup()
    mount({ level: 'all' })
    await user.click(trigger())
    expect(trigger()).toHaveAttribute('aria-expanded', 'true')
    const menu = await screen.findByRole('menu', { name: 'Tuỳ chọn cho Đối soát giao dịch' })
    const group = within(menu).getByRole('group', { name: 'Thông báo' })
    const radios = within(group).getAllByRole('menuitemradio')
    expect(radios.map((r) => r.querySelector('b')?.textContent)).toEqual(['Tất cả', 'Tin chính và thread đang theo dõi', 'Tắt'])
    expect(radios.map((r) => r.getAttribute('aria-checked'))).toEqual(['true', 'false', 'false'])
    expect(within(radios[1]!).getByText('Mặc định')).toBeInTheDocument()
    expect(within(radios[0]!).queryByText('Mặc định')).toBeNull()
  })

  it('says what each level does, as the radio\'s description', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(trigger())
    const off = await screen.findByRole('menuitemradio', { name: /^Tắt/ })
    const id = off.getAttribute('aria-describedby')!
    expect(document.getElementById(id)).toHaveTextContent('Vẫn báo khi có người nhắc tên bạn.')
  })

  it('puts the star and the group info after a divider', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(trigger())
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Gắn sao', 'Thông tin nhóm'])
    expect(menu.querySelector('hr, [role="separator"]')).not.toBeNull()
  })

  it('choosing a level reports it and closes the menu', async () => {
    const user = userEvent.setup()
    const p = mount()
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitemradio', { name: /^Tắt/ }))
    expect(p.onChooseLevel).toHaveBeenCalledWith('off')
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })

  it('choosing the level already set still closes without asking for a change', async () => {
    const user = userEvent.setup()
    const p = mount()
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitemradio', { name: /^Tin chính/ }))
    expect(p.onChooseLevel).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })

  it('stars and unstars from the menu', async () => {
    const user = userEvent.setup()
    const p = mount()
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitem', { name: 'Gắn sao' }))
    expect(p.onToggleStar).toHaveBeenCalledTimes(1)
  })

  it('offers Bỏ gắn sao on a starred conversation, and shows the star after the name with hidden words', async () => {
    const user = userEvent.setup()
    mount({ starred: true })
    expect(within(trigger()).getByText('Đã gắn sao')).toHaveClass('sr-only')
    expect(trigger().querySelector('svg.lucide-star')).not.toBeNull()
    await user.click(trigger())
    expect(await screen.findByRole('menuitem', { name: 'Bỏ gắn sao' })).toBeInTheDocument()
  })

  it('opens the info panel', async () => {
    const user = userEvent.setup()
    const p = mount()
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitem', { name: 'Thông tin nhóm' }))
    expect(p.onOpenInfo).toHaveBeenCalled()
  })

  it('shows a status icon after the name when the level is not the default, with hidden words', () => {
    const { unmount } = render(<ConversationTitleMenu conversation={space} level="off" starred={false} onChooseLevel={vi.fn()} onToggleStar={vi.fn()} onOpenInfo={vi.fn()} />)
    const t = screen.getByRole('button', { name: /Đối soát giao dịch/ })
    expect(within(t).getByText('đã tắt thông báo')).toHaveClass('sr-only')
    expect(t.querySelector('svg.lucide-bell-off')).not.toBeNull()
    unmount()
    render(<ConversationTitleMenu conversation={space} level="all" starred={false} onChooseLevel={vi.fn()} onToggleStar={vi.fn()} onOpenInfo={vi.fn()} />)
    const t2 = screen.getByRole('button', { name: /Đối soát giao dịch/ })
    expect(within(t2).getByText('thông báo mọi tin')).toHaveClass('sr-only')
    expect(t2.querySelector('svg.lucide-bell-ring')).not.toBeNull()
  })

  it('shows no status icon at the default level', () => {
    mount()
    expect(trigger().querySelector('svg.lucide-bell-off, svg.lucide-bell-ring')).toBeNull()
  })

  it('on a direct message offers Bật and Tắt, never Tất cả, and calls it "Thông tin"', async () => {
    const user = userEvent.setup()
    mount({ conversation: dm })
    await user.click(trigger())
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitemradio').map((r) => r.querySelector('b')?.textContent)).toEqual(['Bật', 'Tắt'])
    expect(within(menu).queryByText('Tất cả')).toBeNull()
    expect(within(menu).getByRole('menuitem', { name: 'Thông tin' })).toBeInTheDocument()
  })

  it('returns focus to the name when Esc closes the menu', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(trigger())
    await screen.findByRole('menu')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
    expect(trigger()).toHaveFocus()
  })

  it('moves focus to the checked level when the menu opens', async () => {
    const user = userEvent.setup()
    mount({ level: 'off' })
    await user.click(trigger())
    await waitFor(() => expect(screen.getByRole('menuitemradio', { name: /^Tắt/ })).toHaveFocus())
  })

  it('on a phone the name opens the info screen instead of a menu', async () => {
    const user = userEvent.setup()
    const p = mount({ phone: true })
    expect(trigger()).not.toHaveAttribute('aria-haspopup')
    await user.click(trigger())
    expect(p.onOpenInfo).toHaveBeenCalled()
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('still shows the status icon and the star on a phone', () => {
    mount({ phone: true, level: 'off', starred: true })
    expect(trigger().querySelector('svg.lucide-bell-off')).not.toBeNull()
    expect(trigger().querySelector('svg.lucide-star')).not.toBeNull()
  })
})
