import { screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { approvalApi } from '../../api/approval'
import { ApprovalCard } from './ApprovalCard'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { buildDirectory } from '../../lib/people'
import { CONTACTS } from '../../test/chat-fixtures'
import { REQ_ID, approvalDetail } from '../../test/chat-cards-fixtures'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/approval', async (orig) => {
  const real = await orig<typeof import('../../api/approval')>()
  return { ...real, approvalApi: { ...real.approvalApi, getRequest: vi.fn() } }
})
vi.mock('../../hooks/usePeople', () => ({ usePeople: () => ({ ...buildDirectory(CONTACTS), isLoading: false }) }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws', workspaces: [], isLoading: false }) }))

beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

describe('ApprovalCard', () => {
  it('shows the request title, who submitted it, its status in words and a way to open it', async () => {
    vi.mocked(approvalApi.getRequest).mockResolvedValue(approvalDetail())
    renderWithClient(<ApprovalCard requestId={REQ_ID} />)
    expect(await screen.findByText('Thanh toán phí liên ngân hàng tháng 10')).toBeInTheDocument()
    expect(screen.getByText('Chờ Trần Minh Đức duyệt')).toBeInTheDocument()
    expect(screen.getByText(/Lê Thị Hoa/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Xem đề nghị' })).toBeInTheDocument()
    expectNoIds(document.body.querySelector('[data-card]') as HTMLElement)
  })

  it('shows the outcome of a finished request', async () => {
    vi.mocked(approvalApi.getRequest).mockResolvedValue(approvalDetail({ status: 'approved' }))
    renderWithClient(<ApprovalCard requestId={REQ_ID} />)
    expect(await screen.findByText('Đã duyệt')).toBeInTheDocument()
  })

  it('on a 403 gives only "không xem được", with no title and no link', async () => {
    vi.mocked(approvalApi.getRequest).mockRejectedValue(new ApiError('forbidden', 403))
    renderWithClient(<ApprovalCard requestId={REQ_ID} />)
    expect(await screen.findByText('Đề nghị không xem được')).toBeInTheDocument()
    expect(screen.getByText('Bạn không có quyền xem đề nghị này.')).toBeInTheDocument()
    expect(screen.queryByText(/Thanh toán/)).toBeNull()
    expect(screen.queryByRole('link')).toBeNull()
  })
})
