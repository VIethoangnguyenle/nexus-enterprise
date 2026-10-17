import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AgendaList } from './AgendaList'
import { OCCURRENCES, NOW } from '../../test/calendar-fixtures'
import type { Occurrence } from '../../api/calendar'

// jsdom has no layout, so this reads the structure the phone depends on: nothing
// that is unconditionally a fixed-width column may sit before the title.
describe('the agenda on a phone', () => {
  function renderAgenda() {
    return render(
      <AgendaList
        from={new Date(NOW.getFullYear(), NOW.getMonth(), 12)} days={7} occurrences={OCCURRENCES} now={NOW}
        keyOf={(o: Occurrence) => `${o.eventId}|${o.occurrenceStart}`} colorOf={() => 0} onOpen={() => {}}
      />,
    )
  }
  const unconditional = (cls: string) => cls.split(/\s+/).filter((c) => !c.includes(':'))

  it('gives the title the width of the row, not a share left over by a fixed time column', () => {
    renderAgenda()
    const title = screen.getAllByText('Phỏng vấn kế toán viên').find((el) => el.hasAttribute('data-agenda-title'))!
    const row = title.closest('[data-event]')!
    expect(unconditional(row.className).some((c) => /grid-cols-\[.*rem_1fr/.test(c) || c.includes('8rem'))).toBe(false)
    expect(unconditional(row.className).join(' ')).toContain('grid-cols-[0.875rem_minmax(0,1fr)_auto]')
    expect(row.className).toContain('sm:grid-cols-[0.875rem_8rem_1fr_auto]')
  })

  it('stacks the day heading over its rows instead of beside them', () => {
    renderAgenda()
    const section = screen.getAllByRole('region')[0]!
    expect(unconditional(section.className)).toContain('grid-cols-1')
    expect(unconditional(section.className).some((c) => c.includes('7.5rem'))).toBe(false)
  })

  it('puts the title before the time and shows the place under it', () => {
    renderAgenda()
    const title = screen.getAllByText('Phỏng vấn kế toán viên').find((el) => el.hasAttribute('data-agenda-title'))!
    expect(title.className).toContain('max-sm:order-1')
    const when = title.parentElement!.querySelector('[data-agenda-when]')!
    expect(when.className).toContain('max-sm:order-2')
    expect(when.textContent).toContain('Phòng họp 3')
  })
})
