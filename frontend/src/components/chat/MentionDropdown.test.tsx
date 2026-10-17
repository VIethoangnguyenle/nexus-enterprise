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
