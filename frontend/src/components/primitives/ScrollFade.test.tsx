import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ScrollFade } from './ScrollFade'

function measure(scrollWidth: number, clientWidth: number) {
  vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockReturnValue(scrollWidth)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(clientWidth)
}

afterEach(() => vi.restoreAllMocks())

describe('ScrollFade', () => {
  it('fades the far edge while there is more to scroll to', () => {
    measure(420, 350)
    render(<ScrollFade data-testid="row">tabs</ScrollFade>)
    expect(screen.getByTestId('row')).toHaveAttribute('data-fade', 'end')
  })

  it('fades nothing when everything fits', () => {
    measure(300, 350)
    render(<ScrollFade data-testid="row">tabs</ScrollFade>)
    expect(screen.getByTestId('row')).not.toHaveAttribute('data-fade')
  })

  it('moves the fade to the near edge once scrolled to the end', () => {
    measure(420, 350)
    render(<ScrollFade data-testid="row">tabs</ScrollFade>)
    const row = screen.getByTestId('row')
    row.scrollLeft = 70
    fireEvent.scroll(row)
    expect(row).toHaveAttribute('data-fade', 'start')
  })
})
