import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { expectNoIds } from '../../test/no-ids'
import { CONTACTS, CH, ME, U } from '../../test/chat-fixtures'
import type { Message } from '../../api/messaging'
import { DeleteMessageDialog } from './DeleteMessageDialog'

const people = buildDirectory(CONTACTS)
const message = (over: Partial<Message> = {}): Message => ({
  id: '44444444-aaaa-4bbb-8ccc-0000000000aa', channel_id: CH.doisoat, sender_id: ME.id, sender_name: 'hoa',
  content: 'Tin cần xoá', content_format: 'plain', ...over,
})

function show(m: Message | null, props: Partial<Parameters<typeof DeleteMessageDialog>[0]> = {}) {
  const onConfirm = vi.fn()
  const onCancel = vi.fn()
  renderWithClient(
    <DeleteMessageDialog message={m} me={ME.id} people={people} busy={false} onConfirm={onConfirm} onCancel={onCancel} {...props} />,
  )
  return { onConfirm, onCancel }
}

describe('DeleteMessageDialog', () => {
  it('shows nothing without a message', () => {
    show(null)
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })

  it('is an alert dialog that says what deleting your own message does', () => {
    show(message())
    const dialog = screen.getByRole('alertdialog', { name: 'Xoá tin nhắn?' })
    expect(dialog).toHaveAccessibleDescription('Tin sẽ bị xoá với mọi người trong cuộc trò chuyện và không khôi phục được.')
    expect(within(dialog).getByRole('button', { name: 'Xoá' })).toBeInTheDocument()
  })

  it("names the sender of someone else's message and says the group log records it", () => {
    show(message({ sender_id: U.lan.id, sender_name: 'lan' }))
    const dialog = screen.getByRole('alertdialog')
    expect(dialog).toHaveTextContent('Tin của Nguyễn Thu Lan sẽ bị xoá với mọi người')
    expect(dialog).toHaveTextContent('Nhật ký nhóm ghi lại bạn đã xoá tin này.')
    expectNoIds()
  })

  it('says the thread stays when the message has one', () => {
    show(message({ reply_count: 2 }))
    expect(screen.getByRole('alertdialog')).toHaveTextContent('Thread của tin vẫn giữ lại.')
  })

  it('starts with focus on Huỷ so a stray Enter deletes nothing', () => {
    show(message())
    return vi.waitFor(() => expect(screen.getByRole('button', { name: 'Huỷ' })).toHaveFocus())
  })

  it('confirms with Xoá and cancels with Huỷ or Esc', async () => {
    const user = userEvent.setup()
    const { onConfirm, onCancel } = show(message())
    await user.click(screen.getByRole('button', { name: 'Xoá' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    await user.keyboard('{Escape}')
    expect(onCancel).toHaveBeenCalledTimes(2)
  })

  it('cannot be confirmed twice while the request is out', () => {
    show(message(), { busy: true })
    expect(screen.getByRole('button', { name: /Xoá/ })).toBeDisabled()
  })
})
