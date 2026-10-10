import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { Toaster, toast, useToastStore } from './Toast'

afterEach(() => {
  cleanup()
  useToastStore.getState().clear()
})

describe('Toaster', () => {
  it('sits above the phone bottom bar and its safe-area inset, and at the corner where there is none', () => {
    toast('Đã lưu')
    const { container } = render(<Toaster />)
    const region = container.querySelector('[aria-live="polite"]')!
    expect(region.className).toContain('max-lg:bottom-above-bar')
    expect(region.className).toContain('bottom-4')
  })
})
