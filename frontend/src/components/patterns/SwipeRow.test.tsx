import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pointer, swipe } from '../../test/gesture-events'
import { LONG_PRESS_MS, useLongPress } from '../../hooks/useLongPress'
import { Pressable } from '../primitives'
import { ACTION_WIDTH, SwipeRow, SwipeRowGroup, releaseOf, type SwipeAction } from './SwipeRow'

let reduced = false
vi.mock('../../lib/motion', async () => {
  const actual = await vi.importActual<typeof import('../../lib/motion')>('../../lib/motion')
  return { ...actual, useMotionPresets: () => actual.presets(reduced) }
})

const WIDTH = 400

const nextFrame = () => act(() => new Promise<void>((r) => requestAnimationFrame(() => requestAnimationFrame(() => r()))))

const act1 = (key: string, label: string, onAction = vi.fn()): SwipeAction => ({
  key, label, icon: <span />, tone: 'muted', onAction,
})

/** A Chat row: Tắt thông báo · Gắn sao · Thêm on the left swipe, Đã đọc on the right, full swipe acts. */
function chatRow(id = 'a', onOpen = vi.fn()) {
  const actions = {
    mute: act1('mute', 'Tắt thông báo'),
    star: act1('star', 'Gắn sao'),
    more: act1('more', 'Thêm'),
    read: act1('read', 'Đã đọc'),
  }
  const el = (
    <SwipeRow id={id} fullSwipe start={[actions.read]} end={[actions.mute, actions.star, actions.more]}>
      <Pressable onClick={onOpen}>Hàng {id}</Pressable>
    </SwipeRow>
  )
  return { actions, el, onOpen }
}

/** The element the finger drags: the row content's wrapper. */
const surface = (name: string) => screen.getByRole('button', { name }).parentElement!

beforeEach(() => {
  reduced = false
  Object.defineProperty(window, 'innerWidth', { value: WIDTH, configurable: true, writable: true })
})
afterEach(() => vi.useRealTimers())

describe('releaseOf', () => {
  const base = { width: WIDTH, startWidth: ACTION_WIDTH, endWidth: ACTION_WIDTH * 3, velocity: 0 }

  it('opens a side past half of its buttons, and closes short of it', () => {
    expect(releaseOf({ ...base, offset: -120, fullSwipe: false })).toEqual({ kind: 'open', side: 'end' })
    expect(releaseOf({ ...base, offset: -100, fullSwipe: false })).toEqual({ kind: 'close' })
    expect(releaseOf({ ...base, offset: 40, fullSwipe: false })).toEqual({ kind: 'open', side: 'start' })
    expect(releaseOf({ ...base, offset: 30, fullSwipe: false })).toEqual({ kind: 'close' })
  })

  it('acts on a full swipe right only where the row allows it', () => {
    expect(releaseOf({ ...base, offset: 250, fullSwipe: true })).toEqual({ kind: 'act' })
    // A decision (Phê duyệt): the same swipe only leaves the button revealed.
    expect(releaseOf({ ...base, offset: 250, fullSwipe: false })).toEqual({ kind: 'open', side: 'start' })
  })

  it('never acts on a full swipe left, even with full swipe on', () => {
    expect(releaseOf({ ...base, offset: -390, fullSwipe: true })).toEqual({ kind: 'open', side: 'end' })
  })

  it('follows a flick over the distance', () => {
    expect(releaseOf({ ...base, offset: -20, velocity: -0.8, fullSwipe: false })).toEqual({ kind: 'open', side: 'end' })
    expect(releaseOf({ ...base, offset: -200, velocity: 0.8, fullSwipe: false })).toEqual({ kind: 'close' })
  })

  it('does nothing toward a side without buttons', () => {
    expect(releaseOf({ ...base, startWidth: 0, offset: 300, fullSwipe: true })).toEqual({ kind: 'close' })
  })
})

describe('SwipeRow', () => {
  it('keeps the hidden buttons out of reach until the row opens', () => {
    render(chatRow().el)
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Đã đọc' })).toBeNull()
  })

  it('a swipe left reveals Tắt thông báo · Gắn sao · Thêm; a press runs one and closes the row', () => {
    const { actions, el, onOpen } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 160, t: 300 }])
    expect(onOpen).not.toHaveBeenCalled()
    const star = screen.getByRole('button', { name: 'Gắn sao' })
    expect(screen.getByRole('button', { name: 'Tắt thông báo' })).toBeInTheDocument()
    fireEvent.click(star)
    expect(actions.star.onAction).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
  })

  it('a full swipe right runs Đã đọc at once', () => {
    const { actions, el } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 330, t: 600 }])
    expect(actions.read.onAction).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Đã đọc' })).toBeNull()
  })

  it('a short slow swipe right only reveals Đã đọc', () => {
    const { actions, el } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 110, t: 600 }])
    expect(actions.read.onAction).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Đã đọc' })).toBeInTheDocument()
  })

  it('never starts in the 20px edge zone: that swipe is the back', () => {
    const { actions, el } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 10, t: 0 }, { x: 24, t: 16 }, { x: 330, t: 600 }])
    expect(actions.read.onAction).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Đã đọc' })).toBeNull()
  })

  it('locks to the vertical axis after 10px: a scroll never drags the row', () => {
    const { actions, el } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 100, y: 100, t: 0 }, { x: 102, y: 115, t: 16 }, { x: 390, y: 120, t: 600 }])
    expect(actions.read.onAction).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Đã đọc' })).toBeNull()
  })

  it('ignores a mouse: the desktop has the row menu', () => {
    const { actions, el } = chatRow()
    render(el)
    swipe(surface('Hàng a'), [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 330, t: 600 }], { pointerType: 'mouse' })
    expect(actions.read.onAction).not.toHaveBeenCalled()
  })

  it('swallows the click that ends a swipe, and a tap on an open row only closes it', () => {
    const { el, onOpen } = chatRow()
    render(el)
    const row = surface('Hàng a')
    swipe(row, [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 160, t: 300 }])
    fireEvent.click(screen.getByRole('button', { name: 'Hàng a' }))
    expect(onOpen).not.toHaveBeenCalled()
    // The row is open: a tap closes it and opens nothing.
    pointer(row, 'pointerdown', { x: 100, t: 1000 })
    pointer(row, 'pointerup', { x: 100, t: 1050 })
    fireEvent.click(screen.getByRole('button', { name: 'Hàng a' }))
    expect(onOpen).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
    // Closed, a tap opens the row as usual.
    fireEvent.click(screen.getByRole('button', { name: 'Hàng a' }))
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('renders the row bare when disabled (the desktop)', () => {
    const { container } = render(
      <SwipeRow id="a" disabled end={[act1('x', 'Thêm')]}><span>Hàng</span></SwipeRow>,
    )
    expect(container.innerHTML).toBe('<span>Hàng</span>')
  })

  it('follows the finger 1:1 while dragging', async () => {
    const { el } = chatRow()
    render(el)
    const row = surface('Hàng a')
    pointer(row, 'pointerdown', { x: 300, t: 0 })
    pointer(row, 'pointermove', { x: 288, t: 16 })
    pointer(row, 'pointermove', { x: 200, t: 300 })
    await waitFor(() => expect(row.style.transform).toContain('translateX(-100px)'))
  })

  it('under reduced motion does not follow the finger, but a release past the threshold opens at once', async () => {
    reduced = true
    const { el } = chatRow()
    render(el)
    const row = surface('Hàng a')
    pointer(row, 'pointerdown', { x: 300, t: 0 })
    pointer(row, 'pointermove', { x: 288, t: 16 })
    pointer(row, 'pointermove', { x: 160, t: 300 })
    await nextFrame()
    expect(row.style.transform === '' || row.style.transform === 'none').toBe(true)
    pointer(row, 'pointerup', { x: 160, t: 310 })
    expect(screen.getByRole('button', { name: 'Gắn sao' })).toBeInTheDocument()
    await waitFor(() => expect(row.style.transform).toContain(`translateX(-${ACTION_WIDTH * 3}px)`))
  })
})

describe('Phê duyệt rows: a gesture never decides', () => {
  function approvalRow() {
    const approve = act1('approve', 'Duyệt…')
    const reject = act1('reject', 'Từ chối…')
    render(
      <SwipeRow id="r1" start={[{ ...approve, tone: 'success', width: 92 }]} end={[{ ...reject, tone: 'danger' }]}>
        <Pressable>Mua 3 laptop</Pressable>
      </SwipeRow>,
    )
    return { approve, reject }
  }

  it('a full swipe right leaves Duyệt… revealed and approves nothing', () => {
    const { approve, reject } = approvalRow()
    swipe(surface('Mua 3 laptop'), [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 395, t: 600 }])
    expect(approve.onAction).not.toHaveBeenCalled()
    expect(reject.onAction).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Duyệt…' })).toBeInTheDocument()
  })

  it('a full swipe left leaves Từ chối… revealed and rejects nothing', () => {
    const { approve, reject } = approvalRow()
    swipe(surface('Mua 3 laptop'), [{ x: 395, t: 0 }, { x: 383, t: 16 }, { x: 30, t: 600 }])
    expect(approve.onAction).not.toHaveBeenCalled()
    expect(reject.onAction).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Từ chối…' })).toBeInTheDocument()
  })

  it('only a press on the revealed button asks, through the row’s own confirmation', () => {
    const { approve } = approvalRow()
    swipe(surface('Mua 3 laptop'), [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 395, t: 600 }])
    fireEvent.click(screen.getByRole('button', { name: 'Duyệt…' }))
    expect(approve.onAction).toHaveBeenCalledTimes(1)
  })
})

describe('SwipeRowGroup', () => {
  it('keeps one row open at a time', () => {
    const a = chatRow('a')
    const b = chatRow('b')
    render(<SwipeRowGroup>{a.el}{b.el}</SwipeRowGroup>)
    swipe(surface('Hàng a'), [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 160, t: 300 }])
    expect(screen.getAllByRole('button', { name: 'Gắn sao' })).toHaveLength(1)
    swipe(surface('Hàng b'), [{ x: 300, t: 1000 }, { x: 288, t: 1016 }, { x: 160, t: 1300 }])
    const open = screen.getAllByRole('button', { name: 'Gắn sao' })
    expect(open).toHaveLength(1)
    expect(within(surface('Hàng b').parentElement!).getByRole('button', { name: 'Gắn sao' })).toBe(open[0])
  })

  it('closes the open row when anything scrolls', () => {
    const a = chatRow('a')
    render(<SwipeRowGroup><div data-testid="list">{a.el}</div></SwipeRowGroup>)
    swipe(surface('Hàng a'), [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 160, t: 300 }])
    expect(screen.getByRole('button', { name: 'Gắn sao' })).toBeInTheDocument()
    act(() => { fireEvent.scroll(screen.getByTestId('list')) })
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
  })
})

describe('SwipeRow beside a long press', () => {
  function Pressed({ onLong }: { onLong: () => void }) {
    const press = useLongPress(onLong)
    return (
      <div {...press}>
        <SwipeRow id="a" end={[act1('star', 'Gắn sao')]}>
          <Pressable>Hàng a</Pressable>
        </SwipeRow>
      </div>
    )
  }

  it('a swipe is never a long press, and a still press is never a swipe', () => {
    vi.useFakeTimers()
    const onLong = vi.fn()
    render(<Pressed onLong={onLong} />)
    const row = surface('Hàng a')
    pointer(row, 'pointerdown', { x: 300, t: 0 })
    pointer(row, 'pointermove', { x: 288, t: 16 })
    pointer(row, 'pointermove', { x: 160, t: 300 })
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS + 50) })
    pointer(row, 'pointerup', { x: 160, t: 500 })
    expect(onLong).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Gắn sao' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Gắn sao' }))
    pointer(row, 'pointerdown', { x: 200, t: 1000 })
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS + 50) })
    pointer(row, 'pointerup', { x: 200, t: 1500 })
    expect(onLong).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
  })
})
