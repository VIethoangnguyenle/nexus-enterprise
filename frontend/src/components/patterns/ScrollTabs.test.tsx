import '@testing-library/jest-dom/vitest'
import { cleanup, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from '@testing-library/react'
import { ScrollTabs } from './ScrollTabs'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const tabs = [
  { id: 'all', label: 'Tất cả' },
  { id: 'recent', label: 'Gần đây' },
  { id: 'shared', label: 'Đã chia sẻ' },
]

describe('ScrollTabs', () => {
  it('renders all tab labels', () => {
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={vi.fn()} />)
    expect(screen.getByRole('tab', { name: 'Tất cả' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Gần đây' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Đã chia sẻ' })).toBeInTheDocument()
  })

  it('marks the active tab with aria-selected', () => {
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={vi.fn()} />)
    expect(screen.getByRole('tab', { name: 'Tất cả' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Gần đây' })).toHaveAttribute('aria-selected', 'false')
  })

  it('calls onTabChange when a tab is clicked', async () => {
    const onTabChange = vi.fn()
    const user = userEvent.setup()
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={onTabChange} />)
    await user.click(screen.getByRole('tab', { name: 'Gần đây' }))
    expect(onTabChange).toHaveBeenCalledWith('recent')
  })

  it('does not call onTabChange when the active tab is clicked', async () => {
    const onTabChange = vi.fn()
    const user = userEvent.setup()
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={onTabChange} />)
    await user.click(screen.getByRole('tab', { name: 'Tất cả' }))
    expect(onTabChange).not.toHaveBeenCalled()
  })

  it('uses a tablist role', () => {
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={vi.fn()} />)
    expect(screen.getByRole('tablist')).toBeInTheDocument()
  })

  it('applies active styling to the selected tab', () => {
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={vi.fn()} />)
    const activeTab = screen.getByRole('tab', { name: 'Tất cả' })
    expect(activeTab.className).toContain('font-semibold')
  })

  it('accepts a label for the tablist', () => {
    render(<ScrollTabs tabs={tabs} activeTab="all" onTabChange={vi.fn()} label="Phê duyệt" />)
    expect(screen.getByRole('tablist', { name: 'Phê duyệt' })).toBeInTheDocument()
  })
})
