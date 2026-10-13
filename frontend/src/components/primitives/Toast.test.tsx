import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useUiStore } from '../../stores/ui.store'
import { Toaster, toast, useToastStore } from './Toast'

afterEach(() => {
  cleanup()
  useToastStore.getState().clear()
  useUiStore.setState({ chatBarShown: false })
})

const region = () => {
  toast('Đã lưu')
  const { container } = render(<Toaster />)
  return container.querySelector('[aria-live="polite"]')!
}

describe('Toaster', () => {
  it('keeps 16px and the safe-area inset off the bottom edge on a phone, and sits at the corner on a desktop', () => {
    const el = region()
    expect(el.className).toContain('max-lg:bottom-[calc(1rem+env(safe-area-inset-bottom,0px))]')
    expect(el.className).toContain('bottom-4')
    expect(el).not.toHaveAttribute('data-above-bar')
  })

  it('floats 16px above the Chat bar while it is on screen', () => {
    useUiStore.setState({ chatBarShown: true })
    const el = region()
    expect(el.className).toContain('max-lg:bottom-[calc(6.375rem+env(safe-area-inset-bottom,0px))]')
    expect(el).toHaveAttribute('data-above-bar', 'true')
  })
})
