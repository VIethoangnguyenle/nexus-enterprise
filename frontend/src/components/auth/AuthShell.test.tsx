import { act, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { updatePreferences } from '../../lib/preferences'
import { expectNoIds } from '../../test/no-ids'
import { Button } from '../primitives'
import { AuthShell } from './AuthShell'
import { GRAPH_SETTLE_SECONDS, LOGO_SEEN_KEY } from './auth-motion'

const assembling = (c: HTMLElement) => Array.from(c.querySelectorAll('svg.nexus-mark')).map((s) => s.hasAttribute('data-assemble'))

describe('AuthShell', () => {
  it('puts the screen in the main landmark', () => {
    render(<AuthShell><Button>Tiếp tục</Button></AuthShell>)
    expect(screen.getByRole('main')).toContainElement(screen.getByRole('button', { name: 'Tiếp tục' }))
  })

  it('keeps the brand pane out of the reading order and out of the tab order', () => {
    const { container } = render(<AuthShell><span>x</span></AuthShell>)
    const brand = container.querySelector('aside')!
    expect(brand).toHaveAttribute('aria-hidden', 'true')
    expect(brand.querySelector('a,button,input,[tabindex]')).toBeNull()
  })

  it('hides the pane below 1024px and shows the mark above the form instead', () => {
    const { container } = render(<AuthShell><span>x</span></AuthShell>)
    expect(container.querySelector('aside')!.className).toMatch(/\bhidden\b.*lg:grid|lg:grid.*\bhidden\b/)
    expect(screen.getByRole('main').querySelector('.lg\\:hidden')).not.toBeNull()
  })

  it('shows the product idea with people’s names and no identifier', () => {
    const { container } = render(<AuthShell><span>x</span></AuthShell>)
    expect(container.textContent).toMatch(/Mỗi thay đổi đều mang tên và màu của người làm/)
    expectNoIds(container)
  })

  it('names the product "Nexus" in the brand pane and above the mobile form', () => {
    const { container } = render(<AuthShell><span>x</span></AuthShell>)
    expect(container.textContent).not.toMatch(/Nexus Hub/)
    expect(container.querySelector('aside')!.textContent).toMatch(/Nexus/)
    expect(screen.getByRole('main').textContent).toMatch(/Nexus/)
  })

  describe('the logo', () => {
    beforeEach(() => sessionStorage.clear())

    it('assembles the first time in a browser session, on both marks', () => {
      const { container } = render(<AuthShell><span>x</span></AuthShell>)
      expect(assembling(container)).toEqual([true, true])
    })

    it('is at rest on every later sign-in screen of the session', () => {
      sessionStorage.setItem(LOGO_SEEN_KEY, '1')
      const { container } = render(<AuthShell><span>x</span></AuthShell>)
      expect(assembling(container)).toEqual([false, false])
    })
  })

  describe('the graph behind the brand pane', () => {
    it('is there, decoration only, inside the pane', () => {
      const { container } = render(<AuthShell><span>x</span></AuthShell>)
      const graph = container.querySelector('aside [data-auth-graph]')!
      expect(graph).not.toBeNull()
      expect(graph).toHaveAttribute('aria-hidden', 'true')
    })

    it('is absent under reduced motion', () => {
      updatePreferences({ motion: 'reduce' })
      try {
        const { container } = render(<AuthShell><span>x</span></AuthShell>)
        expect(container.querySelector('[data-auth-graph]')).toBeNull()
      } finally {
        updatePreferences({ motion: 'system' })
      }
    })

    it('stands still while the tab is hidden and goes on when it is back', () => {
      const { container } = render(<AuthShell><span>x</span></AuthShell>)
      const graph = () => container.querySelector('[data-auth-graph]')!
      expect(graph()).not.toHaveAttribute('data-paused')
      const hide = (hidden: boolean) => {
        Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden })
        act(() => { document.dispatchEvent(new Event('visibilitychange')) })
      }
      try {
        hide(true)
        expect(graph()).toHaveAttribute('data-paused', 'true')
        hide(false)
        expect(graph()).not.toHaveAttribute('data-paused')
      } finally {
        Object.defineProperty(document, 'hidden', { configurable: true, get: () => false })
      }
    })

    it('drifts once for about 30s and settles: no layer repeats', () => {
      const { container } = render(<AuthShell><span>x</span></AuthShell>)
      const layers = Array.from(container.querySelectorAll<SVGGElement>('[data-auth-graph] g'))
      expect(layers).toHaveLength(2)
      for (const g of layers) {
        expect(g.style.animationDuration).toBe(`${GRAPH_SETTLE_SECONDS}s`)
        expect(g.style.animationIterationCount).toBe('1')
        expect(g.style.animationFillMode).toBe('forwards')
      }
      expect(GRAPH_SETTLE_SECONDS).toBe(30)
    })
  })
})
