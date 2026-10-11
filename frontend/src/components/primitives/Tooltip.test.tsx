import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Tooltip } from './Tooltip'

function setup(props: { disabled?: boolean } = {}) {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
  const btn = () => screen.getByRole('button', { name: 'Mở rộng' })
  render(
    <div>
      <Tooltip content="Mở rộng menu chính" {...props}>
        {(p) => <button type="button" aria-label="Mở rộng" {...p} />}
      </Tooltip>
      <button type="button">Khác</button>
    </div>,
  )
  return { user, btn }
}

beforeEach(() => localStorage.clear())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('Tooltip', () => {
  it('shows after 400ms of hover, not before', () => {
    const { btn } = setup()
    fireEvent.mouseEnter(btn())
    act(() => { vi.advanceTimersByTime(399) })
    expect(screen.queryByRole('tooltip')).toBeNull()
    act(() => { vi.advanceTimersByTime(2) })
    expect(screen.getByRole('tooltip')).toHaveTextContent('Mở rộng menu chính')
    expect(screen.getByRole('button', { name: 'Mở rộng' })).toHaveAccessibleDescription('Mở rộng menu chính')
  })

  it('does not show when the pointer leaves first', () => {
    const { btn } = setup()
    fireEvent.mouseEnter(btn())
    act(() => { vi.advanceTimersByTime(200) })
    fireEvent.mouseLeave(btn())
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('shows at once on keyboard focus and hides on blur', async () => {
    const { user } = setup()
    await user.tab()
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    await user.tab()
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('does not show for focus that a mouse press caused', () => {
    const { btn } = setup()
    fireEvent.pointerDown(btn())
    fireEvent.mouseDown(btn())
    act(() => btn().focus())
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('hides on Esc', async () => {
    const { user } = setup()
    await user.tab()
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('hides when the button is pressed', () => {
    const { btn } = setup()
    fireEvent.mouseEnter(btn())
    act(() => { vi.advanceTimersByTime(500) })
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    fireEvent.mouseDown(btn())
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('never shows when disabled, and does not describe the button', async () => {
    const { user, btn } = setup({ disabled: true })
    await user.tab()
    fireEvent.mouseEnter(btn())
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByRole('tooltip')).toBeNull()
    expect(screen.getByRole('button', { name: 'Mở rộng' })).not.toHaveAttribute('aria-describedby')
  })
})
