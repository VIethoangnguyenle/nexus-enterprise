import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, MESSAGES, MSG_IMG1, MSG_NO_TEXT, CH, ME } from '../../test/chat-fixtures'
import { pointer } from '../../test/gesture-events'
import { settingsApi } from '../../api/settings'
import { driveApi } from '../../api/drive'
import type { Message } from '../../api/messaging'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { ConversationStream } from './ConversationStream'

vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  useNavigate: () => vi.fn(),
}))

const people = buildDirectory(CONTACTS)

beforeEach(() => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  vi.spyOn(driveApi, 'preview').mockResolvedValue({ kind: 'native', url: 'https://s.test/x.jpg', mime: 'image/jpeg' })
  vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://s.test/dl' })
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
})

function open(messages: Message[], onToggleReaction = vi.fn()) {
  renderWithClient(
    <ConversationStream
      channelId={CH.doisoat}
      workspaceId="ws-test"
      label="Tin nhắn trong Đối soát giao dịch"
      messages={messages}
      ready
      hasMore={false}
      loadingOlder={false}
      onLoadOlder={vi.fn()}
      me={ME.id}
      people={people}
      unread={{ ready: true, lastReadId: '', count: 0 }}
      onOpenThread={vi.fn()}
      onReact={vi.fn()}
      onToggleReaction={onToggleReaction}
      onPin={vi.fn()}
      onFollow={vi.fn()}
    />,
  )
  return onToggleReaction
}

function doubleTap(el: Element) {
  pointer(el, 'pointerdown', { x: 120, t: 1000 })
  pointer(el, 'pointerup', { x: 120, t: 1040 })
  pointer(el, 'pointerdown', { x: 120, t: 1150 })
  pointer(el, 'pointerup', { x: 120, t: 1190 })
}

describe('ConversationStream on a phone', () => {
  it('double-tap gives the reaction the person chose in Cài đặt Chat for this workspace', async () => {
    const get = vi.spyOn(settingsApi, 'get').mockResolvedValue({ double_tap: '🙏' })
    const onToggle = open([MESSAGES[1]!])
    await waitFor(() => expect(get).toHaveBeenCalledWith('chat', 'ws-test'))
    const text = await screen.findByText(/lệch 3 giao dịch/)
    await waitFor(() => {
      doubleTap(text)
      expect(onToggle).toHaveBeenCalledWith(MESSAGES[1]!.id, '🙏', false)
    })
  })

  it('double-tap does nothing when the person turned it off', async () => {
    vi.spyOn(settingsApi, 'get').mockResolvedValue({ double_tap: '' })
    const onToggle = open([MESSAGES[1]!])
    const text = await screen.findByText(/lệch 3 giao dịch/)
    // The answer is in: an empty reaction now means off, not still loading.
    await waitFor(() => expect(queryClient.getQueryData(keys.settings.scope('chat', 'ws-test'))).toEqual({ double_tap: '' }))
    doubleTap(text)
    expect(onToggle).not.toHaveBeenCalled()
  })

  it('the viewer walks every picture of the conversation, oldest first', async () => {
    vi.spyOn(settingsApi, 'get').mockResolvedValue({})
    open([MSG_IMG1, MSG_NO_TEXT])
    await userEvent.click(screen.getByRole('button', { name: /Mở ảnh hinh-anh\.jpg/ }))
    const viewer = await screen.findByRole('dialog')
    expect(within(viewer).getByLabelText('Tệp 2 trên 2')).toBeInTheDocument()
    await userEvent.click(within(viewer).getAllByRole('button', { name: 'Tệp trước' })[0]!)
    expect(within(viewer).getByLabelText('Tệp 1 trên 2')).toBeInTheDocument()
  })
})
