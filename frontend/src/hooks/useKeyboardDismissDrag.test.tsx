import { render, screen } from '@testing-library/react'
import { useRef } from 'react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useKeyboardDismissDrag, DISMISS_DISTANCE } from './useKeyboardDismissDrag'
import { touchSwipe } from '../test/gesture-events'

function List() {
  const ref = useRef<HTMLDivElement>(null)
  useKeyboardDismissDrag(ref)
  return (
    <>
      <div ref={ref} data-testid="list">tin nhắn</div>
      <div role="textbox" aria-label="Ô soạn" tabIndex={0} contentEditable suppressContentEditableWarning />
    </>
  )
}

const original = Object.getOwnPropertyDescriptor(window, 'visualViewport')
function keyboard(open: boolean) {
  Object.defineProperty(window, 'innerWidth', { value: 390, configurable: true, writable: true })
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true, writable: true })
  Object.defineProperty(window, 'visualViewport', {
    configurable: true,
    value: { height: open ? 480 : 800, addEventListener: () => {}, removeEventListener: () => {} },
  })
}

function setup(open = true) {
  keyboard(open)
  render(<List />)
  const field = screen.getByRole('textbox', { name: 'Ô soạn' })
  field.focus()
  return { field, list: screen.getByTestId('list') }
}

beforeEach(() => keyboard(true))
afterEach(() => {
  if (original) Object.defineProperty(window, 'visualViewport', original)
  else Reflect.deleteProperty(window, 'visualViewport')
})

describe('useKeyboardDismissDrag', () => {
  it('puts the keyboard away once the list is dragged down past 48px', () => {
    const { field, list } = setup()
    touchSwipe(list, [{ x: 100, y: 200, t: 1 }, { x: 101, y: 230, t: 20 }])
    expect(field).toHaveFocus()
    touchSwipe(list, [{ x: 100, y: 200, t: 100 }, { x: 102, y: 200 + DISMISS_DISTANCE + 2, t: 140 }])
    expect(field).not.toHaveFocus()
  })

  it('keeps the keyboard for a drag up (reading newer messages)', () => {
    const { field, list } = setup()
    touchSwipe(list, [{ x: 100, y: 300, t: 1 }, { x: 100, y: 200, t: 40 }])
    expect(field).toHaveFocus()
  })

  it('keeps the keyboard for a sideways drag (a message swipe)', () => {
    const { field, list } = setup()
    touchSwipe(list, [{ x: 100, y: 200, t: 1 }, { x: 180, y: 220, t: 20 }, { x: 200, y: 260, t: 40 }])
    expect(field).toHaveFocus()
  })

  it('does nothing while the keyboard is down', () => {
    const { field, list } = setup(false)
    touchSwipe(list, [{ x: 100, y: 200, t: 1 }, { x: 100, y: 300, t: 40 }])
    expect(field).toHaveFocus()
  })
})
