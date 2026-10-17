import '@testing-library/jest-dom/vitest'
import { cleanup, render } from '@testing-library/react'
import { useRef } from 'react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { Pressable } from '../components/primitives'
import { useModalFocus } from './useModalFocus'

function Layer() {
  const ref = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef: ref, onClose: () => {} })
  return <div ref={ref} aria-modal="true" role="dialog"><Pressable>x</Pressable></div>
}

let root: HTMLElement
beforeEach(() => {
  root = document.createElement('div')
  root.id = 'root'
  document.body.appendChild(root)
})
afterEach(() => {
  cleanup()
  root.remove()
})

describe('useModalFocus: layers over the app', () => {
  it('makes the app inert while a layer is open and lets it go when the last one closes', () => {
    const a = render(<Layer />)
    expect(root.inert).toBe(true)
    a.unmount()
    expect(root.inert).toBe(false)
  })

  it('keeps the app inert while any layer is open: closing the upper of two does not free the page', () => {
    const lower = render(<Layer />)
    const upper = render(<Layer />)
    upper.unmount()
    expect(root.inert).toBe(true)
    lower.unmount()
    expect(root.inert).toBe(false)
  })
})
