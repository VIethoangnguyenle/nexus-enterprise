import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { queryClient } from '../../lib/query-client'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, MESSAGES, MSG, THREAD, U, CH, ME } from '../../test/chat-fixtures'
import type { Message } from '../../api/messaging'
import { TopicStream } from './TopicStream'
import { keys } from '../../hooks/keys'

vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

afterEach(resetClient)

const people = buildDirectory(CONTACTS)
const ascending = [...MESSAGES].reverse()

function stream(messages: Message[], props: Partial<Parameters<typeof TopicStream>[0]> = {}) {
  return (
    <TopicStream
      channelId={CH.doisoat}
      messages={messages}
      ready
      me={ME.id}
      people={people}
      onOpenThread={vi.fn()}
      onReact={vi.fn()}
      onToggleReaction={vi.fn()}
      onPin={vi.fn()}
      {...props}
    />
  )
}

const topic = (name: RegExp) => screen.getByRole('article', { name })

describe('TopicStream: topic cards and reply summary', () => {
  it('renders each top-level message as a topic, author by display name', () => {
    renderWithClient(stream(ascending))
    expect(screen.getAllByRole('article')).toHaveLength(2)
    expect(within(topic(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/)).toBeInTheDocument()
    expect(within(topic(/Lê Thị Hoa/)).getByText(/hạch toán vào 6427/)).toBeInTheDocument()
  })

  it('summarises replies from reply_count and opens the thread', async () => {
    const user = userEvent.setup()
    const onOpenThread = vi.fn()
    renderWithClient(stream(ascending, { onOpenThread }))
    const summary = within(topic(/Trần Minh Đức/)).getByRole('button', { name: /2 trả lời/ })
    await user.click(summary)
    expect(onOpenThread).toHaveBeenCalledWith(MSG.t1)
  })

  it('offers "Trả lời" on a topic with no replies yet', async () => {
    const user = userEvent.setup()
    const onOpenThread = vi.fn()
    renderWithClient(stream(ascending, { onOpenThread }))
    const card = topic(/Lê Thị Hoa/)
    expect(within(card).queryByText(/trả lời$/)).toBeNull()
    await user.click(within(card).getByRole('button', { name: 'Trả lời' }))
    expect(onOpenThread).toHaveBeenCalledWith(MSG.t2)
  })

  it('shows who replied and when, once the thread is known', () => {
    queryClient.setQueryData(keys.messaging.thread(MSG.t1), { messages: THREAD })
    renderWithClient(stream(ascending))
    const summary = within(topic(/Trần Minh Đức/)).getByRole('button', { name: /2 trả lời/ })
    expect(within(summary).getByTitle('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(summary).getByTitle('Lê Quang Vinh')).toBeInTheDocument()
    expect(summary).toHaveTextContent('Lần cuối 09:20')
  })
})

describe('TopicStream: realtime attribution', () => {
  const fromYen: Message = {
    id: '44444444-aaaa-4bbb-8ccc-0000000000aa', channel_id: CH.doisoat, sender_id: U.yen.id, sender_name: 'yen',
    content: 'Em đã gửi bản scan phụ lục số 3', content_format: 'plain', created_at: new Date().toISOString(),
  }
  const fromMe: Message = {
    id: '44444444-aaaa-4bbb-8ccc-0000000000bb', channel_id: CH.doisoat, sender_id: ME.id, sender_name: 'hoa',
    content: 'Chị nhận rồi nhé', content_format: 'plain', created_at: new Date().toISOString(),
  }

  it('does not wash history that was there on load', () => {
    renderWithClient(stream(ascending))
    for (const card of screen.getAllByRole('article')) expect(card.className).not.toContain('rt-wash')
  })

  it('washes a topic from someone else in their hue and labels it', () => {
    const view = renderWithClient(stream(ascending))
    view.rerender(stream([...ascending, fromYen]))
    const card = topic(/Phạm Hải Yến/)
    expect(card.className).toContain('rt-wash')
    expect(card.getAttribute('style')).toMatch(/--pw: var\(--color-person-\d-wash\)/)
    expect(within(card).getByText('vừa gửi')).toBeInTheDocument()
  })

  it('gives your own new topic only the insert, no wash and no label', () => {
    const view = renderWithClient(stream(ascending))
    view.rerender(stream([...ascending, fromMe]))
    const card = topic(/Lê Thị Hoa.*Chị nhận rồi/s)
    expect(card.className).not.toContain('rt-wash')
    expect(within(card).queryByText('vừa gửi')).toBeNull()
  })
})
