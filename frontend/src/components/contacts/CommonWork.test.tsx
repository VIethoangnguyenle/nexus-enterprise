import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch } from '../../api/client'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CommonWork } from './CommonWork'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const WS = '44444444-aaaa-4bbb-8ccc-000000000001'
const PERSON = { userId: '11111111-aaaa-4bbb-8ccc-000000000003', nodeId: '22222222-aaaa-4bbb-8ccc-000000000003' }
const GROUPS = [
  { id: '33333333-aaaa-4bbb-8ccc-000000000001', name: 'general', channel_type: 'workspace', workspace_id: WS },
  { id: '33333333-aaaa-4bbb-8ccc-000000000002', name: 'Đối soát liên khối', channel_type: 'private', workspace_id: WS },
]
const FILES = [
  { id: '66666666-aaaa-4bbb-8ccc-000000000001', name: 'Báo cáo tháng 9.xlsx', item_type: 'file', workspace_id: WS, parent_id: '66666666-aaaa-4bbb-8ccc-0000000000ff' },
  { id: '66666666-aaaa-4bbb-8ccc-000000000002', name: 'Hợp đồng', item_type: 'folder', workspace_id: WS },
]

function answer({ groups = GROUPS, files = FILES, failGroups = false }: { groups?: unknown[]; files?: unknown[]; failGroups?: boolean } = {}) {
  api.mockImplementation((path: string) => {
    if (path.includes('/channels/in-common')) return failGroups ? Promise.reject(new ApiError('boom', 500)) : Promise.resolve({ channels: groups })
    if (path.includes('/drive/shared-with-me')) return Promise.resolve({ items: files })
    return Promise.reject(new Error(`unexpected ${path}`))
  })
}

beforeEach(() => { api.mockReset() })
afterEach(() => {
  cleanup()
  resetClient()
})

describe('CommonWork (Đã cùng làm việc)', () => {
  it('asks each service about this person in this workspace, and names what came back, never an id', async () => {
    answer()
    renderWithClient(<CommonWork wsId={WS} person={PERSON} onOpenGroup={() => {}} onOpenFile={() => {}} />)
    const section = await screen.findByRole('region', { name: 'Đã cùng làm việc' })
    expect(await within(section).findByRole('button', { name: /2 nhóm chung/ })).toHaveTextContent('general · Đối soát liên khối')
    expect(within(section).getByRole('button', { name: /Tệp đã chia sẻ với bạn/ })).toHaveTextContent('2 tệp')
    expect(api).toHaveBeenCalledWith(`/workspaces/${WS}/channels/in-common?with=${PERSON.nodeId}`)
    expect(api).toHaveBeenCalledWith(`/workspaces/${WS}/drive/shared-with-me?owner=${PERSON.userId}`)
    expectNoIds()
  })

  it('opens the list of groups, and a group opens its conversation', async () => {
    answer()
    const onOpenGroup = vi.fn()
    const user = userEvent.setup()
    renderWithClient(<CommonWork wsId={WS} person={PERSON} onOpenGroup={onOpenGroup} onOpenFile={() => {}} />)
    await user.click(await screen.findByRole('button', { name: /2 nhóm chung/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Nhóm chung' })
    await user.click(within(sheet).getByRole('button', { name: /Đối soát liên khối/ }))
    expect(onOpenGroup).toHaveBeenCalledWith(GROUPS[1]!.id)
  })

  it('opens the list of files, and a file opens where it is', async () => {
    answer()
    const onOpenFile = vi.fn()
    const user = userEvent.setup()
    renderWithClient(<CommonWork wsId={WS} person={PERSON} onOpenGroup={() => {}} onOpenFile={onOpenFile} />)
    await user.click(await screen.findByRole('button', { name: /Tệp đã chia sẻ với bạn/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Tệp đã chia sẻ với bạn' })
    await user.click(within(sheet).getByRole('button', { name: /Báo cáo tháng 9/ }))
    expect(onOpenFile).toHaveBeenCalledWith(expect.objectContaining({ id: FILES[0]!.id }))
  })

  it('leaves out a source with nothing, and the whole block when both have nothing', async () => {
    answer({ groups: [], files: [] })
    renderWithClient(<CommonWork wsId={WS} person={PERSON} onOpenGroup={() => {}} onOpenFile={() => {}} />)
    await waitFor(() => expect(api).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Đã cùng làm việc' })).toBeNull())
    expect(document.body.textContent).not.toMatch(/nhóm chung|Tệp đã chia sẻ/)
  })

  it('a failed source says so with Thử lại, and the other still shows', async () => {
    answer({ failGroups: true })
    const user = userEvent.setup()
    renderWithClient(<CommonWork wsId={WS} person={PERSON} onOpenGroup={() => {}} onOpenFile={() => {}} />)
    const section = await screen.findByRole('region', { name: 'Đã cùng làm việc' })
    expect(await within(section).findByText('Không tải được nhóm chung')).toBeInTheDocument()
    expect(within(section).getByRole('button', { name: /Tệp đã chia sẻ với bạn/ })).toBeInTheDocument()
    answer()
    await user.click(within(section).getByRole('button', { name: 'Thử lại' }))
    expect(await within(section).findByRole('button', { name: /2 nhóm chung/ })).toBeInTheDocument()
  })
})
