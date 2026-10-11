import { beforeEach, describe, expect, it } from 'vitest'
import type { Message } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { chatCardKeys } from '../hooks/keys/chat-cards'
import { quoteFromMessage, quoteMessage, scopeOf, useDraftsStore } from './drafts.store'

const msg = (over: Partial<Message> = {}): Message => ({
  id: 'm1', channel_id: 'c1', sender_id: 'u1', sender_name: 'Lan', content: '<p>Xin chào cả nhà</p>', content_format: 'html', ...over,
})

beforeEach(() => useDraftsStore.getState().reset())

describe('scopeOf', () => {
  it('is the channel for a main-stream message and the thread for a reply', () => {
    expect(scopeOf(msg())).toBe('c1')
    expect(scopeOf(msg({ parent_message_id: 'root' }))).toBe('thread:root')
  })
})

describe('quoteFromMessage', () => {
  it('quotes the text of a message that has text', () => {
    const q = quoteFromMessage(msg())
    expect(q).toMatchObject({ id: 'm1', sender_id: 'u1', sender_name: 'Lan', text: 'Xin chào cả nhà', summary: undefined })
  })

  it('summarises a message that has only pictures, a file, files or a card', () => {
    const img = { drive_file_id: 'f', file_name: 'a.jpg', mime_type: 'image/jpeg', size_bytes: 1 }
    const doc = { drive_file_id: 'g', file_name: 'bao-cao.pdf', mime_type: 'application/pdf', size_bytes: 1 }
    expect(quoteFromMessage(msg({ content: '', attachments: [img, img, img] })).summary).toEqual({ icon: 'image', label: '3 ảnh' })
    expect(quoteFromMessage(msg({ content: '', attachments: [doc] })).summary).toEqual({ icon: 'file', label: 'bao-cao.pdf' })
    expect(quoteFromMessage(msg({ content: '', attachments: [doc, { ...doc, drive_file_id: 'h' }] })).summary).toEqual({ icon: 'file', label: '2 tệp' })
    expect(quoteFromMessage(msg({ content: '', attachments: [img, doc] })).summary).toEqual({ icon: 'file', label: '1 ảnh, 1 tệp' })
    expect(quoteFromMessage(msg({ content: '', linked_entity_type: 'poll', linked_entity_id: 'p' }), { cardTitle: 'Chốt lịch?' }).summary)
      .toEqual({ icon: 'card', label: 'Bình chọn: Chốt lịch?' })
    expect(quoteFromMessage(msg({ content: '', linked_entity_type: 'task', linked_entity_id: 't' })).summary)
      .toEqual({ icon: 'card', label: 'Công việc' })
  })

  it('cuts a long text and never carries ids into the label', () => {
    const q = quoteFromMessage(msg({ content: `<p>${'a'.repeat(400)}</p>` }))
    expect(q.text.length).toBeLessThanOrEqual(161)
    expect(q.text.endsWith('…')).toBe(true)
  })
})

describe('drafts store', () => {
  it('keeps the text and the quote apart per conversation', () => {
    const s = useDraftsStore.getState()
    s.setHtml('c1', '<p>nháp</p>')
    s.setQuote('c1', quoteFromMessage(msg()))
    s.setHtml('c2', '<p>khác</p>')
    expect(useDraftsStore.getState().drafts['c1']).toMatchObject({ html: '<p>nháp</p>', quote: { id: 'm1' } })
    expect(useDraftsStore.getState().drafts['c2']?.quote).toBeUndefined()
  })

  it('choosing another message replaces the quote (one per composer)', () => {
    const s = useDraftsStore.getState()
    s.setQuote('c1', quoteFromMessage(msg()))
    s.setQuote('c1', quoteFromMessage(msg({ id: 'm2' })))
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('m2')
  })

  it('drops an emptied draft entirely, quote included only when asked', () => {
    const s = useDraftsStore.getState()
    s.setHtml('c1', '<p>x</p>')
    s.setHtml('c1', '')
    expect(useDraftsStore.getState().drafts['c1']).toBeUndefined()
    s.setQuote('c1', quoteFromMessage(msg()))
    s.setHtml('c1', '')
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('m1')
    s.setQuote('c1', null)
    expect(useDraftsStore.getState().drafts['c1']).toBeUndefined()
  })

  it('marks a pending quote deleted and ignores it for another message', () => {
    const s = useDraftsStore.getState()
    s.setQuote('c1', quoteFromMessage(msg()))
    s.markQuoteDeleted('c1', 'other')
    expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBeUndefined()
    s.markQuoteDeleted('c1', 'm1')
    expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBe(true)
  })

  it('pendingQuoteId is the live quote only', () => {
    const s = useDraftsStore.getState()
    expect(s.pendingQuoteId('c1')).toBeUndefined()
    s.setQuote('c1', quoteFromMessage(msg()))
    expect(useDraftsStore.getState().pendingQuoteId('c1')).toBe('m1')
    s.markQuoteDeleted('c1', 'm1')
    expect(useDraftsStore.getState().pendingQuoteId('c1')).toBeUndefined()
  })

  it('knows which composers are editing', () => {
    const s = useDraftsStore.getState()
    s.setEditing('c1', true)
    expect(useDraftsStore.getState().editing['c1']).toBe(true)
    s.setEditing('c1', false)
    expect(useDraftsStore.getState().editing['c1']).toBeUndefined()
  })
})

describe('quoteMessage', () => {
  it('quotes into the composer where the message lives', () => {
    quoteMessage(msg())
    quoteMessage(msg({ id: 'r1', parent_message_id: 'root' }))
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('m1')
    expect(useDraftsStore.getState().drafts['thread:root']?.quote?.id).toBe('r1')
  })

  it('names a poll by its question once it is loaded', () => {
    queryClient.setQueryData(chatCardKeys.poll('p1'), { id: 'p1', question: 'Họp lúc nào?' })
    quoteMessage(msg({ content: '📊 Poll: Họp lúc nào?', message_type: 'system', linked_entity_type: 'poll', linked_entity_id: 'p1' }))
    // The fallback line the message carries ("📊 Poll: …") is not what is quoted.
    expect(useDraftsStore.getState().drafts['c1']?.quote).toMatchObject({ text: '', summary: { icon: 'card', label: 'Bình chọn: Họp lúc nào?' } })
    queryClient.clear()
  })
})
