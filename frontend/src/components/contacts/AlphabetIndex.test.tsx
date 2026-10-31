import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AlphabetIndex } from './AlphabetIndex'

afterEach(cleanup)

const LETTERS = ['A', 'B', 'D', 'Đ', 'H']
const index = () => screen.getByRole('navigation', { name: 'Nhảy theo chữ cái' })

describe('AlphabetIndex', () => {
  it('is one button per letter, in the order given', () => {
    render(<AlphabetIndex letters={LETTERS} onJump={() => {}} />)
    expect(within(index()).getAllByRole('button').map((b) => b.textContent)).toEqual(LETTERS)
    expect(within(index()).getByRole('button', { name: 'Nhảy tới Đ' })).toBeInTheDocument()
  })

  it('jumps on a tap (and from the keyboard)', async () => {
    const onJump = vi.fn()
    const user = userEvent.setup()
    render(<AlphabetIndex letters={LETTERS} onJump={onJump} />)
    await user.click(within(index()).getByRole('button', { name: 'Nhảy tới D' }))
    expect(onJump).toHaveBeenLastCalledWith('D')
  })

  it('follows a finger dragged down the column, showing the letter large while dragging', () => {
    const onJump = vi.fn()
    render(<AlphabetIndex letters={LETTERS} onJump={onJump} />)
    const column = index()
    // Five letters over 100px: 20px each.
    column.getBoundingClientRect = () => ({ top: 0, bottom: 100, height: 100, left: 0, right: 18, width: 18, x: 0, y: 0, toJSON: () => ({}) })
    act(() => { fireEvent.pointerDown(column, { clientY: 5, pointerId: 1 }) })
    expect(onJump).toHaveBeenLastCalledWith('A')
    expect(screen.getByTestId('az-bubble')).toHaveTextContent('A')
    act(() => { fireEvent.pointerMove(column, { clientY: 65, pointerId: 1 }) })
    expect(onJump).toHaveBeenLastCalledWith('Đ')
    expect(screen.getByTestId('az-bubble')).toHaveTextContent('Đ')
    act(() => { fireEvent.pointerUp(column, { clientY: 65, pointerId: 1 }) })
    expect(screen.queryByTestId('az-bubble')).toBeNull()
    // A move with no finger down does nothing.
    onJump.mockClear()
    act(() => { fireEvent.pointerMove(column, { clientY: 95, pointerId: 1 }) })
    expect(onJump).not.toHaveBeenCalled()
  })

  it('draws nothing without letters', () => {
    render(<AlphabetIndex letters={[]} onJump={() => {}} />)
    expect(screen.queryByRole('navigation')).toBeNull()
  })
})
