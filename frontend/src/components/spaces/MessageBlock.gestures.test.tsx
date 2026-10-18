import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, ME, MESSAGES, MSG_NO_TEXT } from '../../test/chat-fixtures'
import { pointer, swipe } from '../../test/gesture-events'
import { buildDirectory } from '../../lib/people'
import { MessageBlock, type MessageActions } from './MessageBlock'
import type { ViewerFile } from '../files/FileViewer'
import type { Message } from '../../api/messaging'
import { driveApi } from '../../api/drive'

vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => vi.fn(),
}))

const people = buildDirectory(CONTACTS)
/** Someone else's message that you (ME) already reacted 👍 to. */
const theirs: Message = MESSAGES[1]!
const liked: Message = MESSAGES[0]!
/** React reads a zero `timeStamp` as missing, so gesture time starts at 1s. */
const T0 = 1000

let phone = true
beforeEach(() => {
  phone = true
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://s.test/dl' })
  vi.spyOn(driveApi, 'preview').mockResolvedValue({ kind: 'native', url: 'https://s.test/x', mime: 'image/jpeg' })
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
})

function block(m: Message, extra: Partial<MessageActions> & { gallery?: ViewerFile[] } = {}) {
  const actions = {
    onReact: vi.fn(),
    onToggleReaction: vi.fn(),
    onReply: vi.fn(),
    onQuote: vi.fn(),
    onOpenSheet: vi.fn(),
    onEdit: vi.fn(),
    onMore: vi.fn(),
    onPin: vi.fn(),
    ...extra,
  }
  renderWithClient(<MessageBlock message={m} people={people} idBase="g" me={ME.id} quickReaction="👍" {...actions} />)
  return { ...actions, bubble: document.getElementById('g-text')! }
}

const right = (from = 120) => [{ x: from, t: T0 }, { x: from + 15, t: T0 + 20 }, { x: from + 80, t: T0 + 80 }]
const left = (from = 300) => [{ x: from, t: T0 }, { x: from - 15, t: T0 + 20 }, { x: from - 80, t: T0 + 80 }]

describe('MessageBlock on a phone: swipes', () => {
  it('quotes on a swipe right and opens the thread on a swipe left, for someone else’s message', () => {
    const a = block(theirs)
    swipe(a.bubble, right())
    expect(a.onQuote).toHaveBeenCalledWith(theirs)
    swipe(a.bubble, left())
    expect(a.onReply).toHaveBeenCalledWith(theirs.id)
  })

  it('uses the same directions for your own message', () => {
    const a = block(liked)
    swipe(a.bubble, right())
    swipe(a.bubble, left())
    expect(a.onQuote).toHaveBeenCalledWith(liked)
    expect(a.onReply).toHaveBeenCalledWith(liked.id)
  })

  it('never edits, deletes or opens the menu by a swipe, however far', () => {
    const a = block(liked)
    swipe(a.bubble, [{ x: 40, t: T0 }, { x: 55, t: T0 + 20 }, { x: 380, t: T0 + 80 }])
    swipe(a.bubble, [{ x: 380, t: T0 }, { x: 365, t: T0 + 20 }, { x: 30, t: T0 + 80 }])
    expect(a.onEdit).not.toHaveBeenCalled()
    expect(a.onOpenSheet).not.toHaveBeenCalled()
    expect(a.onMore).not.toHaveBeenCalled()
    expect(a.onToggleReaction).not.toHaveBeenCalled()
  })

  it('leaves a swipe from the 20px edge to the back gesture; 40px in it quotes', () => {
    const a = block(theirs)
    swipe(a.bubble, right(10))
    expect(a.onQuote).not.toHaveBeenCalled()
    swipe(a.bubble, right(40))
    expect(a.onQuote).toHaveBeenCalledTimes(1)
  })

  it('shows the cues: ↩ on the quote side, the thread disc on the other', () => {
    block(theirs)
    expect(document.querySelector('[data-swipe-cue="quote"]')).toBeInTheDocument()
    expect(document.querySelector('[data-swipe-cue="thread"]')).toBeInTheDocument()
  })

  it('has no thread swipe on a call card, which has no thread', () => {
    const call: Message = { ...theirs, linked_entity_type: 'call', linked_entity_id: 'c1', content: 'Cuộc gọi' }
    const a = block(call)
    expect(document.querySelector('[data-swipe-cue="thread"]')).toBeNull()
    const el = document.querySelector('[data-message-swipe] > div')!
    swipe(el, left())
    expect(a.onReply).not.toHaveBeenCalled()
  })

  it('does not swipe a message still being sent', () => {
    const a = block({ ...theirs, _optimistic: true } as Message)
    expect(document.querySelector('[data-message-swipe]')).toBeNull()
    swipe(a.bubble, right())
    expect(a.onQuote).not.toHaveBeenCalled()
  })

  it('keeps the wide screen as it was: no swipe wrapper, no swipe', () => {
    phone = false
    const a = block(theirs)
    expect(document.querySelector('[data-message-swipe]')).toBeNull()
    swipe(a.bubble, right())
    expect(a.onQuote).not.toHaveBeenCalled()
  })
})

describe('MessageBlock on a phone: long press and slide', () => {
  function pressAndHold(bubble: Element) {
    vi.useFakeTimers()
    pointer(bubble, 'pointerdown', { x: 120, y: 500, t: T0 })
    act(() => {
      vi.advanceTimersByTime(400)
    })
    const row = screen.getByRole('toolbar', { name: 'Thả cảm xúc' })
    within(row).getAllByRole('button').forEach((b, i) => {
      b.getBoundingClientRect = () => new DOMRect(20 + i * 56, 300, 44, 44)
    })
    return row
  }

  it('slides onto a reaction and reacts on release, without the sheet', () => {
    const a = block(theirs)
    pressAndHold(a.bubble)
    pointer(a.bubble, 'pointermove', { x: 90, y: 400, t: T0 + 450 })
    pointer(a.bubble, 'pointermove', { x: 90, y: 320, t: T0 + 500 })
    pointer(a.bubble, 'pointerup', { x: 90, y: 320, t: T0 + 520 })
    expect(a.onToggleReaction).toHaveBeenCalledWith(theirs.id, '❤️', false)
    expect(a.onOpenSheet).not.toHaveBeenCalled()
    expect(screen.queryByRole('toolbar', { name: 'Thả cảm xúc' })).toBeNull()
  })

  it('takes a reaction you already gave away again', () => {
    const a = block(liked)
    pressAndHold(a.bubble)
    pointer(a.bubble, 'pointermove', { x: 40, y: 320, t: T0 + 450 })
    pointer(a.bubble, 'pointerup', { x: 40, y: 320, t: T0 + 470 })
    expect(a.onToggleReaction).toHaveBeenCalledWith(liked.id, '👍', true)
  })

  it('opens the sheet as before when the finger lifts before sliding', () => {
    const a = block(theirs)
    pressAndHold(a.bubble)
    pointer(a.bubble, 'pointerup', { x: 121, y: 501, t: T0 + 450 })
    expect(a.onOpenSheet).toHaveBeenCalledWith(theirs)
    expect(a.onToggleReaction).not.toHaveBeenCalled()
  })

  it('cancels when the finger slides off the row and lifts', () => {
    const a = block(theirs)
    pressAndHold(a.bubble)
    pointer(a.bubble, 'pointermove', { x: 90, y: 320, t: T0 + 450 })
    pointer(a.bubble, 'pointermove', { x: 90, y: 700, t: T0 + 480 })
    pointer(a.bubble, 'pointerup', { x: 90, y: 700, t: T0 + 500 })
    expect(a.onToggleReaction).not.toHaveBeenCalled()
    expect(a.onOpenSheet).not.toHaveBeenCalled()
    expect(screen.queryByRole('toolbar', { name: 'Thả cảm xúc' })).toBeNull()
  })

  it('goes straight to the sheet where reactions are not offered (a thread)', () => {
    const a = block(theirs, { onReact: undefined })
    vi.useFakeTimers()
    pointer(a.bubble, 'pointerdown', { x: 120, y: 500, t: T0 })
    act(() => {
      vi.advanceTimersByTime(400)
    })
    expect(a.onOpenSheet).toHaveBeenCalledWith(theirs)
    expect(screen.queryByRole('toolbar', { name: 'Thả cảm xúc' })).toBeNull()
  })

  it('a slide after the press is not a swipe', () => {
    const a = block(theirs)
    pressAndHold(a.bubble)
    pointer(a.bubble, 'pointermove', { x: 140, y: 500, t: T0 + 450 })
    pointer(a.bubble, 'pointermove', { x: 220, y: 500, t: T0 + 480 })
    pointer(a.bubble, 'pointerup', { x: 220, y: 500, t: T0 + 500 })
    expect(a.onQuote).not.toHaveBeenCalled()
  })
})

describe('MessageBlock on a phone: double-tap', () => {
  const doubleTap = (el: Element) => {
    pointer(el, 'pointerdown', { x: 120, t: T0 })
    pointer(el, 'pointerup', { x: 120, t: T0 + 40 })
    pointer(el, 'pointerdown', { x: 121, t: T0 + 150 })
    pointer(el, 'pointerup', { x: 121, t: T0 + 190 })
  }

  it('gives the quick reaction', () => {
    const a = block(theirs)
    doubleTap(a.bubble)
    expect(a.onToggleReaction).toHaveBeenCalledWith(theirs.id, '👍', false)
  })

  it('takes it away again when you already gave it', () => {
    const a = block(liked)
    doubleTap(a.bubble)
    expect(a.onToggleReaction).toHaveBeenCalledWith(liked.id, '👍', true)
  })

  it('gives the reaction the person chose', () => {
    const onToggleReaction = vi.fn()
    renderWithClient(
      <MessageBlock message={theirs} people={people} idBase="g" me={ME.id} quickReaction="❤️" onReact={vi.fn()} onToggleReaction={onToggleReaction} />,
    )
    doubleTap(document.getElementById('g-text')!)
    expect(onToggleReaction).toHaveBeenCalledWith(theirs.id, '❤️', false)
  })

  it('does nothing when the person turned it off', () => {
    const onToggleReaction = vi.fn()
    renderWithClient(
      <MessageBlock message={theirs} people={people} idBase="g" me={ME.id} quickReaction="" onReact={vi.fn()} onToggleReaction={onToggleReaction} />,
    )
    doubleTap(document.getElementById('g-text')!)
    expect(onToggleReaction).not.toHaveBeenCalled()
  })

  it('does nothing on a wide screen', () => {
    phone = false
    const a = block(theirs)
    doubleTap(a.bubble)
    expect(a.onToggleReaction).not.toHaveBeenCalled()
  })

  it('does nothing on a deleted message', () => {
    const a = block({ ...theirs, deleted: true })
    const el = document.querySelector('#g-text')!
    doubleTap(el)
    expect(a.onToggleReaction).not.toHaveBeenCalled()
  })
})

describe('MessageBlock on a phone: every gesture has an action', () => {
  it('names Trích dẫn, Trả lời trong thread and Thả cảm xúc for a screen reader', async () => {
    const a = block(theirs)
    const group = screen.getByRole('group', { name: 'Thao tác nhanh với tin' })
    await userEvent.click(within(group).getByRole('button', { name: 'Trích dẫn' }))
    expect(a.onQuote).toHaveBeenCalledWith(theirs)
    await userEvent.click(within(group).getByRole('button', { name: 'Trả lời trong thread' }))
    expect(a.onReply).toHaveBeenCalledWith(theirs.id)
    await userEvent.click(within(group).getByRole('button', { name: 'Thả cảm xúc' }))
    expect(a.onReact).toHaveBeenCalledWith(theirs.id)
  })

  it('offers only the actions that exist', () => {
    block(theirs, { onReply: undefined, onReact: undefined })
    const group = screen.getByRole('group', { name: 'Thao tác nhanh với tin' })
    expect(within(group).getAllByRole('button').map((b) => b.textContent)).toEqual(['Trích dẫn'])
  })
})

describe('MessageBlock: the viewer walks the conversation’s pictures', () => {
  const other: ViewerFile = { id: '55555555-aaaa-4bbb-8ccc-000000000099', name: 'anh-hop.jpg', mime: 'image/jpeg', uploaderName: 'Lan' }
  const mine: ViewerFile = { id: MSG_NO_TEXT.attachments![0]!.drive_file_id, name: 'hinh-anh.jpg', mime: 'image/jpeg', uploaderName: 'Lan' }

  it('opens a picture at its place among all of them', async () => {
    block(MSG_NO_TEXT, { gallery: [other, mine] })
    await userEvent.click(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ }))
    const viewer = await screen.findByRole('dialog')
    expect(within(viewer).getByLabelText('Tệp 2 trên 2')).toBeInTheDocument()
  })

  it('walks only the message’s own files without a gallery', async () => {
    block(MSG_NO_TEXT)
    await userEvent.click(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ }))
    const viewer = await screen.findByRole('dialog')
    expect(within(viewer).queryByLabelText(/^Tệp \d+ trên/)).toBeNull()
  })
})
