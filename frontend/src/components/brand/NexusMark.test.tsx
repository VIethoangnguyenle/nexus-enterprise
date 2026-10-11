import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { updatePreferences } from '../../lib/preferences'
import { NexusLogo } from './NexusLogo'
import { NexusMark } from './NexusMark'

describe('NexusMark', () => {
  it('reads as "Nexus" to assistive technology when it stands alone', () => {
    render(<NexusMark />)
    expect(screen.getByRole('img', { name: 'Nexus' })).toBeInTheDocument()
  })

  it('is hidden from assistive technology when the name is written next to it', () => {
    const { container } = render(<NexusMark decorative />)
    expect(screen.queryByRole('img')).toBeNull()
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('draws one path through four nodes, the last in the person colour', () => {
    const { container } = render(<NexusMark />)
    expect(container.querySelectorAll('[data-nexus-edge]')).toHaveLength(3)
    const nodes = container.querySelectorAll('[data-nexus-node]')
    expect(nodes).toHaveLength(4)
    expect(nodes[3]).toHaveAttribute('data-nexus-node', 'end')
  })

  it('takes its size from the prop', () => {
    const { container } = render(<NexusMark size={36} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '36')
    expect(svg).toHaveAttribute('height', '36')
  })

  it('uses the heavy small drawing at 24px and under: stroke only, with the end node', () => {
    const { container } = render(<NexusMark size={16} />)
    expect(container.querySelectorAll('[data-nexus-node]')).toHaveLength(1)
    expect(container.querySelector('[data-nexus-node]')).toHaveAttribute('data-nexus-node', 'end')
    expect(container.querySelector('path')).toHaveAttribute('stroke-width', '8')
  })

  it('assembles only when asked', () => {
    const { container, rerender } = render(<NexusMark />)
    expect(container.querySelector('svg')).not.toHaveAttribute('data-assemble')
    rerender(<NexusMark assemble />)
    expect(container.querySelector('svg')).toHaveAttribute('data-assemble', 'true')
  })

  it('has no assembly under reduced motion: a fade, not a build-up', () => {
    updatePreferences({ motion: 'reduce' })
    try {
      const { container } = render(<NexusMark assemble />)
      expect(container.querySelector('svg')).toHaveAttribute('data-reduced', 'true')
    } finally {
      updatePreferences({ motion: 'system' })
    }
  })

  it('on an accent background it is one colour', () => {
    const { container } = render(<NexusMark tone="on-accent" />)
    expect(container.querySelector('svg')).toHaveAttribute('data-tone', 'on-accent')
  })
})

describe('NexusLogo', () => {
  it('writes the name once and keeps the mark out of the reading order', () => {
    render(<NexusLogo />)
    expect(screen.getByText('Nexus')).toBeInTheDocument()
    expect(screen.queryByRole('img')).toBeNull()
    expect(document.body.textContent).not.toMatch(/Hub/)
  })

  it('can carry a line under the name', () => {
    render(<NexusLogo subtitle="Không gian làm việc chung" />)
    expect(screen.getByText('Không gian làm việc chung')).toBeInTheDocument()
  })
})
