import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Contact } from '../../hooks/useContacts'
import { PersonRow } from './PersonRow'

afterEach(cleanup)

const who = (over: Partial<Contact> = {}): Contact => ({
  user_id: '11111111-aaaa-4bbb-8ccc-000000000001', ngac_node_id: '22222222-aaaa-4bbb-8ccc-000000000001', username: 'lan',
  display_name: 'Nguyễn Thu Lan', email: 'lan@novapay.vn', title: 'Chuyên viên đối soát', department: 'Đối soát',
  department_id: 'ds', roles: [], location: '', avatar_url: '', is_online: false, ...over,
})

describe('PersonRow', () => {
  it('shows the name, the title and the status in words', () => {
    render(<PersonRow person={who()} online={false} onSelect={() => {}} />)
    const row = screen.getByRole('button')
    expect(within(row).getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(row).getByText('Chuyên viên đối soát')).toBeInTheDocument()
    expect(within(row).getByText('Ngoại tuyến')).toBeInTheDocument()
  })

  it('says Trực tuyến for someone online', () => {
    render(<PersonRow person={who()} online onSelect={() => {}} />)
    expect(screen.getByText('Trực tuyến')).toBeInTheDocument()
  })

  it('puts the department path after the title, each segment apart, the last one whole', () => {
    render(<PersonRow person={who()} online={false} path={[{ id: 'k', name: 'Khối Vận hành' }, { id: 'ds', name: 'Đối soát' }]} onSelect={() => {}} />)
    const row = screen.getByRole('button')
    expect(within(row).getByText('Chuyên viên đối soát')).toBeInTheDocument()
    expect(within(row).getByText('Khối Vận hành')).toBeInTheDocument()
    // A reader hears the path as a list of steps, not a run of words.
    expect(row.textContent).toContain('Khối Vận hành, Đối soát')
  })

  it('marks the matched words as found', () => {
    render(<PersonRow person={who()} online={false} query="doi soat" onSelect={() => {}} />)
    expect(document.querySelector('mark')?.textContent).toBe('đối soát')
  })

  it('marks the chosen row, and says (bạn) on one\'s own', () => {
    render(<PersonRow person={who()} online={false} selected isMe onSelect={() => {}} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-current', 'true')
    expect(screen.getByText('(bạn)')).toBeInTheDocument()
  })

  it('opens the profile with the row that was pressed, so focus can go back to it', async () => {
    const onSelect = vi.fn()
    const person = who()
    render(<PersonRow person={person} online={false} onSelect={onSelect} />)
    await userEvent.setup().click(screen.getByRole('button'))
    expect(onSelect).toHaveBeenCalledWith(person, screen.getByRole('button'))
  })

  it('shows a person with no display name as an unknown member, never by username', () => {
    render(<PersonRow person={who({ display_name: '', username: 'handle_lan' })} online={false} onSelect={() => {}} />)
    expect(screen.getByText('Thành viên')).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('handle_lan')
  })
})
