import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, CH, ME, U } from '../../test/chat-fixtures'
import type { Message } from '../../api/messaging'
import { ConversationStream } from './ConversationStream'

vi.mock('../chat/PollCard', () => ({ PollCard: ({ pollId }: { pollId: string }) => <div data-testid="poll-card">poll:{pollId}</div> }))
vi.mock('../chat/TaskCard', () => ({ TaskCard: ({ taskId }: { taskId: string }) => <div data-testid="task-card">task:{taskId}</div> }))

afterEach(resetClient)

const people = buildDirectory(CONTACTS)
const msg = (over: Partial<Message>): Message => ({
  id: 'm1', channel_id: CH.doisoat, sender_id: U.lan.id, sender_name: 'lan', content: '', created_at: new Date(2026, 9, 9, 10, 0).toISOString(), ...over,
})

const show = (messages: Message[]) =>
  renderWithClient(
    <ConversationStream
      channelId={CH.doisoat}
      label="Tin nhắn"
      messages={messages}
      ready
      hasMore={false}
      loadingOlder={false}
      onLoadOlder={vi.fn()}
      me={ME.id}
      people={people}
      unread={{ ready: true, lastReadId: '', count: 0 }}
      onOpenThread={vi.fn()}
      onReact={vi.fn()}
      onToggleReaction={vi.fn()}
      onPin={vi.fn()}
      onFollow={vi.fn()}
    />,
  )

describe('ConversationStream: cards that the server posts as system messages', () => {
  it('shows a poll as a card under its creator, not as a "created task" note', () => {
    show([msg({ id: 'p1', message_type: 'system', content: '📊 Poll: Họp lúc nào?', linked_entity_type: 'poll', linked_entity_id: 'poll-1' })])
    const a = screen.getByRole('article', { name: /Nguyễn Thu Lan/ })
    expect(within(a).getByTestId('poll-card')).toHaveTextContent('poll:poll-1')
    expect(screen.queryByText(/đã tạo công việc/)).toBeNull()
  })

  it('shows a task as a card', () => {
    show([msg({ id: 't1', message_type: 'system', content: '📋 Task: Gọi ngân hàng', linked_entity_type: 'task', linked_entity_id: 'task-1' })])
    expect(within(screen.getByRole('article', { name: /Nguyễn Thu Lan/ })).getByTestId('task-card')).toHaveTextContent('task:task-1')
  })

  it('still shows a system line without a card as a quiet note', () => {
    show([msg({ id: 'd1', message_type: 'system', content: 'Trần Minh Đức đã xoá một tin nhắn', linked_entity_type: 'message_deleted' })])
    expect(screen.getByText('Trần Minh Đức đã xoá một tin nhắn')).toBeInTheDocument()
    expect(screen.queryByRole('article')).toBeNull()
  })
})
