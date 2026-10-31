import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, UUID_RE } from '../../test/chat-fixtures'
import { MentionDropdown } from './MentionDropdown'

const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const members = [{ user_id: HIDDEN, username: 'thanh', ngac_node_id: 'n-hidden' }]
const people = buildDirectory(CONTACTS, {
  resolved: {
    byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Nguyễn Hữu Thành', avatarUrl: '' }]]),
    byNodeId: new Map(),
  },
})

describe('MentionDropdown: a member of the group the directory does not list', () => {
  it('is still offered, by the name the resolver found', async () => {
    const onSelect = vi.fn()
    const { container } = render(<MentionDropdown members={members} people={people} query="thành" onSelect={onSelect} onClose={() => {}} />)
    await userEvent.click(screen.getByText('Nguyễn Hữu Thành'))
    expect(onSelect).toHaveBeenCalledWith(members[0])
    expect(container.textContent).not.toMatch(UUID_RE)
  })

  it('is not offered to a query that does not match', () => {
    render(<MentionDropdown members={members} people={people} query="zzz" onSelect={() => {}} onClose={() => {}} />)
    expect(screen.queryByText('Nguyễn Hữu Thành')).toBeNull()
  })
})

describe('MentionDropdown: @Tất cả', () => {
  it('is offered first where the caller allows it, and picked with Enter', async () => {
    const onSelectAll = vi.fn()
    render(<MentionDropdown members={members} people={people} query="" onSelect={() => {}} onSelectAll={onSelectAll} onClose={() => {}} />)
    const options = screen.getAllByRole('option')
    expect(options[0]).toHaveTextContent('@Tất cả')
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
    await userEvent.keyboard('{Enter}')
    expect(onSelectAll).toHaveBeenCalledTimes(1)
  })

  it('answers to "tat" and "all" as typed', () => {
    const { rerender } = render(<MentionDropdown members={members} query="tat" onSelect={() => {}} onSelectAll={() => {}} onClose={() => {}} />)
    expect(screen.getByText('@Tất cả')).toBeInTheDocument()
    rerender(<MentionDropdown members={members} query="all" onSelect={() => {}} onSelectAll={() => {}} onClose={() => {}} />)
    expect(screen.getByText('@Tất cả')).toBeInTheDocument()
    rerender(<MentionDropdown members={members} query="thanh" onSelect={() => {}} onSelectAll={() => {}} onClose={() => {}} />)
    expect(screen.queryByText('@Tất cả')).toBeNull()
  })

  it('is not offered where the caller does not allow it', () => {
    render(<MentionDropdown members={members} people={people} query="" onSelect={() => {}} onClose={() => {}} />)
    expect(screen.queryByText('@Tất cả')).toBeNull()
  })
})
