import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { SpaceView } from './SpaceView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

beforeEach(() => {
  routerState.params = { channelId: CH.doisoat }
  routerState.pathname = `/channels/${CH.doisoat}`
})
afterEach(resetClient)

const noIds = () => expect(document.body.textContent).not.toMatch(UUID_RE)

describe('SpaceView on a phone', () => {
  it('goes back to Trang chủ, the conversation list, since there is no drawer', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: 'Về danh sách trò chuyện' }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' })
  })

  it('marks its search action so the phone top bar can reach it', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    expect(await screen.findByRole('button', { name: 'Tìm trong nhóm' })).toHaveAttribute('data-module-search')
  })
})

describe('SpaceView', () => {
  it('shows the space by name with its member count, and its topics', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    expect(await screen.findByRole('heading', { name: 'Đối soát giao dịch' })).toBeInTheDocument()
    expect(await screen.findByText('14 thành viên')).toBeInTheDocument()
    expect(await screen.findByText(/lệch 3 giao dịch/)).toBeInTheDocument()
    expect(screen.getByText('Mỗi tin gửi ở đây mở một chủ đề mới')).toBeInTheDocument()
    noIds()
  })

  it('renders no identifier anywhere: stream, thread, members, files, tasks', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    noIds()

    // Thread panel
    await user.click(screen.getByRole('button', { name: /2 trả lời/ }))
    const thread = await screen.findByRole('complementary', { name: 'Chủ đề' })
    await within(thread).findByText(/1\.284\.500/)
    noIds()

    // Members panel
    await user.click(screen.getByRole('button', { name: '14 thành viên' }))
    const members = await screen.findByRole('complementary', { name: 'Thành viên' })
    await within(members).findByText('Trần Minh Đức')
    expect(within(members).getByText('Lê Thị Hoa (bạn)')).toBeInTheDocument()
    expect(within(members).getByText('Trưởng phòng · Vận hành thanh toán')).toBeInTheDocument()
    noIds()

    // Files tab
    await user.click(screen.getByRole('tab', { name: 'Tệp' }))
    await screen.findByText('doi-soat-08-10.xlsx')
    expect(screen.getByText('412 KB')).toBeInTheDocument()
    noIds()

    // Tasks tab
    await user.click(screen.getByRole('tab', { name: /Công việc/ }))
    await screen.findByText('Cập nhật biên bản đối soát 08/10')
    const tasks = screen.getByRole('list', { name: 'Công việc' })
    await waitFor(() => expect(within(tasks).getByText('Lê Quang Vinh')).toBeInTheDocument())
    expect(within(tasks).getByText('Cần làm')).toBeInTheDocument()
    noIds()
  })

  it('closes the side panel with Escape', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: /2 trả lời/ }))
    await screen.findByRole('complementary', { name: 'Chủ đề' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Chủ đề' })).toBeNull())
  })
})
