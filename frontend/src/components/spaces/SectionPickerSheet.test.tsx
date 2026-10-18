import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { SectionPickerSheet } from './SectionPickerSheet'

afterEach(resetClient)

describe('SectionPickerSheet', () => {
  const sections = [
    { id: 's1', name: 'Kế toán', position: 0, channel_ids: ['ch-a'] },
    { id: 's2', name: 'Vận hành', position: 1, channel_ids: [] },
  ]

  it('lists all sections with a radio for the current one', async () => {
    renderWithClient(
      <SectionPickerSheet
        sections={sections}
        currentSectionId="s1"
        onPick={vi.fn()}
        onRemove={vi.fn()}
        onClose={vi.fn()}
      />,
    )
    expect(await screen.findByText('Kế toán')).toBeInTheDocument()
    expect(screen.getByText('Vận hành')).toBeInTheDocument()
    // Current section should be checked
    const checked = screen.getByRole('radio', { checked: true })
    expect(checked).toBeInTheDocument()
  })

  it('shows Không có when channelId is in no section', async () => {
    renderWithClient(
      <SectionPickerSheet
        sections={sections}
        currentSectionId={undefined}
        onPick={vi.fn()}
        onRemove={vi.fn()}
        onClose={vi.fn()}
      />,
    )
    const noneRadio = await screen.findByRole('radio', { name: /Không có/ })
    expect(noneRadio).toHaveAttribute('aria-checked', 'true')
  })

  it('calls onPick when a section is chosen', async () => {
    const user = userEvent.setup()
    const onPick = vi.fn()
    renderWithClient(
      <SectionPickerSheet
        sections={sections}
        currentSectionId={undefined}
        onPick={onPick}
        onRemove={vi.fn()}
        onClose={vi.fn()}
      />,
    )
    await user.click(await screen.findByText('Kế toán'))
    expect(onPick).toHaveBeenCalledWith('s1')
  })

  it('calls onRemove when Không có is selected from a section', async () => {
    const user = userEvent.setup()
    const onRemove = vi.fn()
    renderWithClient(
      <SectionPickerSheet
        sections={sections}
        currentSectionId="s1"
        onPick={vi.fn()}
        onRemove={onRemove}
        onClose={vi.fn()}
      />,
    )
    await user.click(await screen.findByRole('radio', { name: /Không có/ }))
    expect(onRemove).toHaveBeenCalled()
  })
})
