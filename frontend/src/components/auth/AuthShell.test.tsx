import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoIds } from '../../test/no-ids'
import { Button } from '../primitives'
import { AuthShell } from './AuthShell'

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
})
