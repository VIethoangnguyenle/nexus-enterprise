import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, ME, U, UUID_RE } from '../../test/chat-fixtures'
import type { PendingQuote } from '../../stores/drafts.store'
import { QuoteStrip } from './QuoteStrip'

const people = buildDirectory(CONTACTS)
const quote = (over: Partial<PendingQuote> = {}): PendingQuote => ({
  id: 'q-1', sender_id: U.lan.id, sender_name: 'lan', text: 'Em thấy kỳ trước để 6427', ...over,
})
const show = (q: PendingQuote, props: Partial<Parameters<typeof QuoteStrip>[0]> = {}) =>
  renderWithClient(<QuoteStrip quote={q} people={people} me={ME.id} onClear={vi.fn()} {...props} />)

describe('QuoteStrip', () => {
  it('is a region named for whose message is quoted, showing their name and one line', () => {
    show(quote())
    const region = screen.getByRole('region', { name: 'Đang trích dẫn tin của Nguyễn Thu Lan' })
    expect(region).toHaveTextContent('Nguyễn Thu Lan')
    expect(region).toHaveTextContent('Em thấy kỳ trước để 6427')
  })

  it('calls your own message "bạn" in its name and "Bạn" on the strip', () => {
    show(quote({ sender_id: ME.id, sender_name: 'hoa' }))
    const region = screen.getByRole('region', { name: 'Đang trích dẫn tin của bạn' })
    expect(region).toHaveTextContent('Bạn')
  })

  it('shows what a message without text holds', () => {
    show(quote({ text: '', summary: { icon: 'image', label: '3 ảnh' } }))
    expect(screen.getByRole('region')).toHaveTextContent('3 ảnh')
  })

  it('takes the quote away with ✕', async () => {
    const user = userEvent.setup()
    const onClear = vi.fn()
    show(quote(), { onClear })
    await user.click(screen.getByRole('button', { name: 'Bỏ trích dẫn' }))
    expect(onClear).toHaveBeenCalledTimes(1)
  })

  it('goes to the original when pressed', async () => {
    const user = userEvent.setup()
    const onGoTo = vi.fn()
    show(quote(), { onGoTo })
    await user.click(screen.getByRole('button', { name: /Nguyễn Thu Lan/ }))
    expect(onGoTo).toHaveBeenCalledTimes(1)
  })

  it('says a deleted message is gone, that the reply goes without it, and no longer links to it', () => {
    show(quote({ deleted: true, text: 'cũ' }), { onGoTo: vi.fn() })
    const region = screen.getByRole('region', { name: 'Đang trích dẫn tin của Nguyễn Thu Lan, tin này đã bị xoá' })
    expect(region).toHaveTextContent('Tin nhắn đã bị xoá')
    expect(region).toHaveTextContent('Tin của bạn sẽ được gửi không kèm trích dẫn.')
    expect(region).not.toHaveTextContent('cũ')
    expect(screen.queryByTitle('Đi tới tin gốc')).toBeNull()
    expect(screen.getByRole('button', { name: 'Bỏ trích dẫn' })).toBeInTheDocument()
  })

  it('shows no id', () => {
    const { container } = show(quote({ id: '11111111-aaaa-4bbb-8ccc-000000000001' }))
    expect(container.innerHTML).not.toMatch(UUID_RE)
  })
})
