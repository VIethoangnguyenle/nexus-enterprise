import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi, type Channel } from '../../api/messaging'
import { ApiError } from '../../api/client'
import { useChannels } from '../../hooks/useMessaging'
import { useCanManageChannel } from '../../hooks/useCanManageChannel'
import { buildConversations } from '../../lib/conversations'
import { EMPTY_DIRECTORY } from '../../lib/people'
import { planFor } from '../../lib/realtime'
import { queryClient } from '../../lib/query-client'
import { CH, ME, SPACES, WS_ID } from '../../test/chat-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { renderWithClient } from '../../test/render'
import { ConversationIcon } from './ConversationIcon'
import { SpaceInfoPanel } from './SpaceInfoPanel'

vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return {
    ...real,
    messagingApi: {
      ...real.messagingApi,
      listPins: vi.fn(),
      listChannels: vi.fn(),
      uploadChannelAvatar: vi.fn(),
      removeChannelAvatar: vi.fn(),
    },
  }
})
vi.mock('../../hooks/useCanManageChannel', () => ({ useCanManageChannel: vi.fn() }))

const PIC = '/api/channels/avatars/0123456789abcdef0123456789abcdef'
const NEXT = '/api/channels/avatars/fedcba9876543210fedcba9876543210'
const TITLE = 'Đối soát giao dịch'
const png = () => new File([new Uint8Array(100)], 'nhom.png', { type: 'image/png' })

function panel(over: Partial<React.ComponentProps<typeof SpaceInfoPanel>> = {}) {
  return (
    <SpaceInfoPanel
      channelId={CH.doisoat}
      kind="space"
      title={TITLE}
      memberCount={14}
      people={EMPTY_DIRECTORY}
      onOpenTopic={() => {}}
      onClose={() => {}}
      {...over}
    />
  )
}
const editButton = () => screen.queryByRole('button', { name: `Đổi ảnh nhóm ${TITLE}` })

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
  vi.mocked(messagingApi.listPins).mockResolvedValue({ pins: [] } as never)
  vi.mocked(useCanManageChannel).mockReturnValue(true)
})

describe('SpaceInfoPanel group picture', () => {
  it('offers the change to someone who manages the group, named after the group', async () => {
    renderWithClient(panel({ avatarUrl: PIC }))
    expect(editButton()).toBeInTheDocument()
    expect(document.querySelector('img')).toHaveAttribute('src', `${PIC}/192`)
    expectNoIds()
  })

  it('shows the picture and no way to change it to anyone else', () => {
    vi.mocked(useCanManageChannel).mockReturnValue(false)
    renderWithClient(panel({ avatarUrl: PIC }))
    expect(editButton()).toBeNull()
    expect(document.querySelector('img')).toHaveAttribute('src', `${PIC}/192`)
  })

  it('has none in a direct message, even for someone who manages it', () => {
    renderWithClient(panel({ kind: 'dm', title: 'Phạm Hải Yến', memberCount: 2 }))
    expect(screen.queryByRole('button', { name: /Đổi ảnh/ })).toBeNull()
  })

  it('sends a chosen picture and shows the address the server answers', async () => {
    vi.mocked(messagingApi.uploadChannelAvatar).mockResolvedValue({ avatar_url: NEXT })
    queryClient.setQueryData(['channels', WS_ID], { channels: SPACES })
    const user = userEvent.setup()
    renderWithClient(panel())

    await user.click(editButton()!)
    await user.click(await screen.findByRole('menuitem', { name: 'Tải ảnh lên' }))
    const file = png()
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy', { selector: 'input' }), file)

    await waitFor(() => expect(messagingApi.uploadChannelAvatar).toHaveBeenCalledWith(CH.doisoat, file))
    const list = queryClient.getQueryData<{ channels: Channel[] }>(['channels', WS_ID])!
    expect(list.channels.find((c) => c.id === CH.doisoat)!.avatar_url).toBe(NEXT)
  })

  it('removes the picture', async () => {
    vi.mocked(messagingApi.removeChannelAvatar).mockResolvedValue({ avatar_url: '' })
    const user = userEvent.setup()
    renderWithClient(panel({ avatarUrl: PIC }))

    await user.click(editButton()!)
    await user.click(await screen.findByRole('menuitem', { name: 'Bỏ ảnh' }))
    await waitFor(() => expect(messagingApi.removeChannelAvatar).toHaveBeenCalledWith(CH.doisoat))
  })

  it("says under the picture what the server refused, in the app's words and not the server's", async () => {
    vi.mocked(messagingApi.uploadChannelAvatar).mockRejectedValue(
      new ApiError('image: decode failed at 0x3f', 400, { message: 'image: decode failed at 0x3f', code: 'avatar_not_image' }),
    )
    const user = userEvent.setup()
    renderWithClient(panel())

    await user.click(editButton()!)
    await user.click(await screen.findByRole('menuitem', { name: 'Tải ảnh lên' }))
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy', { selector: 'input' }), png())

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Chọn ảnh JPG hoặc PNG.')
    expect(alert).not.toHaveTextContent('0x3f')
  })

  it('says the group is not the person’s to change when the server answers 403', async () => {
    vi.mocked(messagingApi.uploadChannelAvatar).mockRejectedValue(new ApiError('forbidden', 403, { message: 'forbidden' }))
    const user = userEvent.setup()
    renderWithClient(panel())

    await user.click(editButton()!)
    await user.click(await screen.findByRole('menuitem', { name: 'Tải ảnh lên' }))
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy', { selector: 'input' }), png())

    expect(await screen.findByRole('alert')).toHaveTextContent('Bạn không có quyền đổi ảnh này.')
  })
})

describe('a group picture changed by someone else', () => {
  function List() {
    const channels = useChannels(WS_ID)
    const [c] = buildConversations({
      spaces: channels.data?.channels ?? [], dms: [], unread: {}, lastMessages: {}, me: ME, dir: EMPTY_DIRECTORY,
    })
    return c ? <ConversationIcon conversation={c} size={40} /> : null
  }

  it('shows in the list once the rename event refreshes the channel lists', async () => {
    const space = { ...SPACES[0]!, id: CH.doisoat }
    vi.mocked(messagingApi.listChannels).mockResolvedValueOnce({ channels: [space] })
    const { container } = renderWithClient(<List />)
    await waitFor(() => expect(container).toHaveTextContent('Đ'))
    expect(container.querySelector('img')).toBeNull()

    vi.mocked(messagingApi.listChannels).mockResolvedValue({ channels: [{ ...space, avatar_url: NEXT }] })
    const plan = planFor({ domain: 'channel', kind: 'renamed', workspaceId: WS_ID, ids: [CH.doisoat], channelId: CH.doisoat })
    for (const key of plan.invalidate) void queryClient.invalidateQueries({ queryKey: key })

    await waitFor(() => expect(container.querySelector('img')).toHaveAttribute('src', `${NEXT}/192`))
    expect(within(container).queryByText('Đ')).toBeNull()
  })
})
