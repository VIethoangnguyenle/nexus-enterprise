import { useRef } from 'react'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { shortcutsApi } from '../../api/shortcuts'
import type { Message } from '../../api/messaging'
import { useDraftsStore } from '../../stores/drafts.store'
import { Button } from '../primitives'
import { MessageMenu } from './MessageMenu'

vi.mock('../../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../../api/shortcuts')>()
  return { ...real, shortcutsApi: { ...real.shortcutsApi, starMessage: vi.fn().mockResolvedValue(undefined), unstarMessage: vi.fn().mockResolvedValue(undefined) } }
})

beforeEach(() => useDraftsStore.getState().reset())
afterEach(() => {
  resetClient()
  vi.clearAllMocks()
})

const msg = (over: Partial<Message> = {}): Message => ({
  id: 'm1', channel_id: 'c1', sender_id: 'u-lan', sender_name: 'Lan', content: '<p>Chốt 6427 nhé</p>', content_format: 'html', ...over,
})

function Host({ message }: { message: Message }) {
  const anchor = useRef<HTMLElement | null>(null)
  return (
    <>
      <Button ref={(el) => { anchor.current = el }}>anchor</Button>
      <MessageMenu target={{ message, up: false }} anchorRef={anchor} onClose={vi.fn()} />
    </>
  )
}
const open = (message: Message) => renderWithClient(<Host message={message} />)
const items = () => within(screen.getByRole('menu')).getAllByRole('menuitem').map((i) => i.textContent)

describe('MessageMenu: star', () => {
  it('stars an unstarred message', async () => {
    const user = userEvent.setup()
    open(msg())
    await user.click(screen.getByRole('menuitem', { name: 'Gắn sao tin' }))
    await waitFor(() => expect(shortcutsApi.starMessage).toHaveBeenCalledWith('m1'))
  })

  it('unstars a starred message', async () => {
    const user = userEvent.setup()
    open(msg({ starred: true }))
    expect(screen.queryByRole('menuitem', { name: 'Gắn sao tin' })).toBeNull()
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ gắn sao tin' }))
    await waitFor(() => expect(shortcutsApi.unstarMessage).toHaveBeenCalledWith('m1'))
  })

  it('comes after Theo dõi thread and before the copy', () => {
    renderWithClient(
      <MessageMenu target={{ message: msg(), up: false }} anchorRef={{ current: document.body }} onClose={vi.fn()} onFollow={vi.fn()} />,
    )
    expect(items()).toEqual(['Theo dõi thread', 'Gắn sao tin', 'Sao chép văn bản', 'Trích dẫn khi trả lời'])
  })

  it('is not offered on a message still being sent, a deleted one or a system line', () => {
    for (const m of [msg({ id: 'temp-1' }), msg({ deleted: true }), msg({ message_type: 'system', linked_entity_type: 'message_deleted' })]) {
      const { unmount } = open(m)
      expect(screen.queryByRole('menuitem', { name: /gắn sao tin/ })).toBeNull()
      expect(screen.queryByRole('menuitem', { name: 'Trích dẫn khi trả lời' })).toBeNull()
      unmount()
    }
  })
})

describe('MessageMenu: quote', () => {
  it('sets the quote of the conversation composer for a message of the stream', async () => {
    const user = userEvent.setup()
    open(msg())
    await user.click(screen.getByRole('menuitem', { name: 'Trích dẫn khi trả lời' }))
    expect(useDraftsStore.getState().drafts['c1']?.quote).toMatchObject({ id: 'm1', sender_name: 'Lan', text: 'Chốt 6427 nhé' })
  })

  it('sets the quote of the thread composer for a reply', async () => {
    const user = userEvent.setup()
    open(msg({ id: 'r1', parent_message_id: 'root' }))
    await user.click(screen.getByRole('menuitem', { name: 'Trích dẫn khi trả lời' }))
    expect(useDraftsStore.getState().drafts['thread:root']?.quote?.id).toBe('r1')
    expect(useDraftsStore.getState().drafts['c1']).toBeUndefined()
  })

  it('is not offered while the composer holds a message being edited', () => {
    useDraftsStore.getState().setEditing('c1', true)
    open(msg())
    expect(screen.queryByRole('menuitem', { name: 'Trích dẫn khi trả lời' })).toBeNull()
    expect(screen.getByRole('menuitem', { name: 'Gắn sao tin' })).toBeInTheDocument()
  })
})
