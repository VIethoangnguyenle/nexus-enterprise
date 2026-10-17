import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApprovalAssignment, ApprovalRequest, ApprovalRequestDetail, AuditEntry } from '../../api/approval'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { renderWithClient } from '../../test/render'
import { ApprovalChain } from './ApprovalChain'
import { ApprovalDetailPanel } from './ApprovalDetailPanel'
import { AuditList } from './AuditList'

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

const hooks = vi.hoisted(() => ({ detail: undefined as unknown, audit: undefined as unknown }))
vi.mock('../../hooks/useApproval', () => ({
  useApprovalRequest: () => ({ data: hooks.detail, isError: false, isLoading: false }),
  useApprovalAudit: () => ({ data: hooks.audit, isError: false, isLoading: false }),
}))

const HIDDEN_NODE = '22222222-aaaa-4bbb-8ccc-0000000000ff'
const HIDDEN_USER = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const SENTENCE = 'Hồ sơ này không còn hiển thị với bạn.'

const RESOLVED = { userId: HIDDEN_USER, nodeId: HIDDEN_NODE, name: 'Nguyễn Hữu Thành', avatarUrl: '' }

// Nguyễn Hữu Thành is outside the directory; the resolver names him by node.
const people = buildDirectory(CONTACTS, {
  resolved: {
    byUserId: new Map([[HIDDEN_USER, RESOLVED]]),
    byNodeId: new Map([[HIDDEN_NODE, RESOLVED]]),
  },
})

const request: ApprovalRequest = {
  id: '55555555-aaaa-4bbb-8ccc-000000000001', entity_type: 'expense', entity_id: 'e1', template_id: 't1',
  template_name: 'Tạm ứng công tác phí', form_data_json: '{}', current_step: 1, status: 'pending',
  scope_oa_id: 's', department_id: 'd', created_by: HIDDEN_NODE, created_by_name: 'Nguyễn Hữu Thành',
  created_at: '2026-10-09T01:30:00Z', completed_at: null,
}
const assignment = (over: Partial<ApprovalAssignment>): ApprovalAssignment => ({
  id: 'a1', step_order: 1, step_name: '', user_node_id: U.duc.node, user_name: 'Trần Minh Đức', grant_source: 'direct',
  status: 'pending', acted_at: null, comment: '', ...over,
})
const entry = (over: Partial<AuditEntry>): AuditEntry => ({
  id: 'aaaaaaaa-0000-4000-8000-000000000001', request_id: request.id, action: 'approved', actor_node_id: HIDDEN_NODE,
  actor_name: 'Nguyễn Hữu Thành', step_order: 1, detail_json: '{}', ip_address: '', created_at: '2026-10-09T01:41:00Z', ...over,
})

beforeEach(() => navigate.mockReset())

async function expectCard() {
  const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
  expect(within(card).getByText(SENTENCE)).toBeInTheDocument()
  expect(within(card).queryAllByRole('button')).toHaveLength(0)
  expect(navigate).not.toHaveBeenCalled()
}

describe('ApprovalChain: pressing an approver', () => {
  const items = (userId: string | undefined, name: string) => [{
    key: 'k1', name, hueKey: userId ?? 'x', avatarUrl: '', ...(userId ? { userId } : {}), stepName: 'Trưởng phòng',
    pill: { tone: 'idle' as const, label: 'Đang chờ' }, isMe: false,
  }]

  it('a hidden approver opens the small card', async () => {
    const { container } = renderWithClient(<ApprovalChain items={items(HIDDEN_USER, 'Nguyễn Hữu Thành')} people={people} />)
    await userEvent.click(screen.getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    await expectCard()
    expect(container.textContent).not.toMatch(UUID_RE)
  })

  it('a visible approver opens their profile', async () => {
    renderWithClient(<ApprovalChain items={items(U.duc.id, 'Trần Minh Đức')} people={people} />)
    await userEvent.click(screen.getByRole('button', { name: 'Trần Minh Đức' }))
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })

  it('one control per approver: the avatar is a target but not a tab stop', () => {
    const { container } = renderWithClient(<ApprovalChain items={items(U.duc.id, 'Trần Minh Đức')} people={people} />)
    expect(screen.getAllByRole('button')).toHaveLength(1)
    expect(container.querySelector('button[aria-hidden="true"]')).toHaveAttribute('tabindex', '-1')
  })

  it('keeps "(bạn)" outside the part of the line that is cut short', () => {
    renderWithClient(<ApprovalChain items={[{ ...items(U.duc.id, 'Trần Minh Đức')[0]!, isMe: true }]} people={people} />)
    const me = screen.getByText('(bạn)')
    expect(me.closest('.truncate')).toBeNull()
    expect(me.className).toContain('shrink-0')
  })

  it('a step nobody holds yet, or a role, has nothing to press', () => {
    renderWithClient(<ApprovalChain items={items(undefined, 'Kế toán trưởng')} people={people} />)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
    expect(screen.getByText('Kế toán trưởng')).toBeInTheDocument()
  })
})

describe('AuditList: pressing an actor', () => {
  it('a hidden actor opens the card, and the row still has actor, action and time', async () => {
    hooks.audit = { entries: [entry({})] }
    renderWithClient(<AuditList requestId={request.id} people={people} />)
    const row = screen.getByRole('listitem')
    expect(row).toHaveTextContent('Nguyễn Hữu Thành')
    expect(row).toHaveTextContent('đã duyệt')
    expect(row.querySelector('time')).not.toBeNull()
    await userEvent.click(within(row).getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    await expectCard()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('a visible actor opens their profile', async () => {
    hooks.audit = { entries: [entry({ actor_node_id: U.duc.node, actor_name: 'Trần Minh Đức' })] }
    renderWithClient(<AuditList requestId={request.id} people={people} />)
    await userEvent.click(screen.getByRole('button', { name: 'Trần Minh Đức' }))
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })

  it('a system entry has no actor to press', () => {
    hooks.audit = { entries: [entry({ action: 'step_advanced', actor_node_id: '', actor_name: undefined })] }
    renderWithClient(<AuditList requestId={request.id} people={people} />)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })
})

describe('ApprovalDetailPanel: pressing the requester and the approvers', () => {
  it('a hidden requester opens the small card', async () => {
    hooks.audit = { entries: [] }
    const detail: ApprovalRequestDetail = {
      request, steps: null, form_fields: null, can_act: false,
      assignments: [assignment({})],
    }
    hooks.detail = detail
    renderWithClient(
      <ApprovalDetailPanel
        requestId={request.id} people={people} meNodeId={U.hoa.node} approving={false}
        onApprove={() => {}} onReturn={() => {}} onClose={() => {}}
      />,
    )
    const sender = screen.getByText('Người gửi').nextElementSibling as HTMLElement
    await userEvent.click(within(sender).getByRole('button', { name: 'Nguyễn Hữu Thành' }))
    await expectCard()
    // The approver in the chain is visible and opens a profile.
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('button', { name: 'Trần Minh Đức' }))
    expect(navigate.mock.calls[0]![0]).toMatchObject({ to: '/contacts' })
  })
})
