import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { shortcutsApi } from '../../api/shortcuts'
import type { Message } from '../../api/messaging'
import { buildDirectory } from '../../lib/people'
import { CONTACTS } from '../../test/chat-fixtures'
import { useDraftsStore } from '../../stores/drafts.store'
import { MessageActionSheet } from './MessageActionSheet'

vi.mock('../../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../../api/shortcuts')>()
  return { ...real, shortcutsApi: { ...real.shortcutsApi, starMessage: vi.fn().mockResolvedValue(undefined), unstarMessage: vi.fn().mockResolvedValue(undefined) } }
})

beforeEach(() => useDraftsStore.getState().reset())
afterEach(() => {
  resetClient()
  vi.clearAllMocks()
})

const people = buildDirectory(CONTACTS)
const msg = (over: Partial<Message> = {}): Message => ({
  id: 'm1', channel_id: 'c1', sender_id: 'u-lan', sender_name: 'Lan', content: '<p>Chốt 6427 nhé</p>', content_format: 'html', ...over,
})
const open = (m: Message, props: Partial<Parameters<typeof MessageActionSheet>[0]> = {}) =>
  renderWithClient(<MessageActionSheet message={m} people={people} me="me" onClose={vi.fn()} {...props} />)

describe('MessageActionSheet: star and quote', () => {
  it('lists Gắn sao tin after the follow row and Trích dẫn khi trả lời after the copy', () => {
    open(msg(), { onReply: vi.fn(), onToggleFollow: vi.fn(), onPin: vi.fn() })
    const names = within(screen.getByRole('dialog')).getAllByRole('button').map((b) => b.textContent)
    const at = (n: string) => names.indexOf(n)
    expect(at('Theo dõi thread')).toBeGreaterThan(-1)
    expect(at('Gắn sao tin')).toBe(at('Theo dõi thread') + 1)
    expect(at('Trích dẫn khi trả lời')).toBe(at('Sao chép') + 1)
  })

  it('stars and unstars, closing the sheet', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const { unmount } = open(msg(), { onClose })
    await user.click(screen.getByRole('button', { name: 'Gắn sao tin' }))
    expect(onClose).toHaveBeenCalled()
    await waitFor(() => expect(shortcutsApi.starMessage).toHaveBeenCalledWith('m1'))
    unmount()
    open(msg({ starred: true }))
    await user.click(screen.getByRole('button', { name: 'Bỏ gắn sao tin' }))
    await waitFor(() => expect(shortcutsApi.unstarMessage).toHaveBeenCalledWith('m1'))
  })

  it('quotes into the composer of the message', async () => {
    const user = userEvent.setup()
    open(msg({ id: 'r1', parent_message_id: 'root' }))
    await user.click(screen.getByRole('button', { name: 'Trích dẫn khi trả lời' }))
    expect(useDraftsStore.getState().drafts['thread:root']?.quote).toMatchObject({ id: 'r1', text: 'Chốt 6427 nhé' })
  })

  it('has no Trích dẫn while the phone composer is editing a message, but keeps the star', () => {
    useDraftsStore.getState().setEditing('c1', true)
    open(msg())
    expect(screen.queryByRole('button', { name: 'Trích dẫn khi trả lời' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Gắn sao tin' })).toBeInTheDocument()
  })

  it('offers neither on a deleted message', () => {
    open(msg({ deleted: true }))
    expect(screen.queryByRole('button', { name: /gắn sao tin|Trích dẫn/ })).toBeNull()
  })
})
