import '@testing-library/jest-dom/vitest'
import { cleanup, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from '@testing-library/react'
import { Pressable } from '../primitives'
import { ModuleHeader } from './ModuleHeader'

const mockUseBottomBarLayout = vi.fn(() => true)

vi.mock('../../hooks/usePhone', () => ({
  useBottomBarLayout: () => mockUseBottomBarLayout(),
}))

afterEach(() => {
  cleanup()
})

beforeEach(() => {
  mockUseBottomBarLayout.mockReturnValue(true)
})

// Danh bạ has no ⋯ (no settings page), so these need no router; the ⋯ is tested in ModuleMenu.test.tsx.
describe('ModuleHeader', () => {
  it('renders title on mobile', () => {
    render(<ModuleHeader moduleId="contacts" title="Tài liệu" />)
    expect(screen.getByRole('heading', { name: 'Tài liệu' })).toBeInTheDocument()
  })

  it('renders nothing on desktop', () => {
    mockUseBottomBarLayout.mockReturnValue(false)
    const { container } = render(<ModuleHeader moduleId="contacts" title="Tài liệu" />)
    expect(container.firstElementChild).toBeNull()
  })

  it('renders action buttons on mobile', () => {
    render(
      <ModuleHeader moduleId="contacts" title="Tài liệu" actions={<Pressable>Action</Pressable>} />,
    )
    expect(screen.getByRole('button', { name: 'Action' })).toBeInTheDocument()
  })

  it('renders children (ScrollTabs) below the title row', () => {
    render(
      <ModuleHeader moduleId="contacts" title="Tài liệu">
        <div data-testid="tabs">Tabs here</div>
      </ModuleHeader>,
    )
    expect(screen.getByTestId('tabs')).toBeInTheDocument()
  })

  it('does not render children on desktop', () => {
    mockUseBottomBarLayout.mockReturnValue(false)
    render(
      <ModuleHeader moduleId="contacts" title="Tài liệu">
        <div data-testid="tabs">Tabs here</div>
      </ModuleHeader>,
    )
    expect(screen.queryByTestId('tabs')).toBeNull()
  })

  it('renders the icon when provided', () => {
    const Icon = () => <span data-testid="icon">📄</span>
    render(<ModuleHeader moduleId="contacts" title="Tài liệu" icon={<Icon />} />)
    expect(screen.getByTestId('icon')).toBeInTheDocument()
  })
})
