import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { documentApi } from '../../api/documents'
import { driveApi } from '../../api/drive'
import { messagingApi } from '../../api/messaging'
import { DocDialog } from './DocDialog'
import { renderWithClient, resetClient } from '../../test/render'
import { DOC_ID, textDoc } from '../../test/chat-cards-fixtures'

vi.mock('../../api/documents', async (orig) => {
  const real = await orig<typeof import('../../api/documents')>()
  return { ...real, documentApi: { ...real.documentApi, create: vi.fn() } }
})
vi.mock('../../api/drive', async (orig) => {
  const real = await orig<typeof import('../../api/drive')>()
  return { ...real, driveApi: { ...real.driveApi, channelDrive: vi.fn() } }
})
vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, sendMessage: vi.fn() } }
})

const FOLDER = 'dddddddd-aaaa-4bbb-8ccc-000000000001'
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(driveApi.channelDrive).mockResolvedValue({ items: [{ id: FOLDER, name: 'Biên bản', item_type: 'folder' }] } as never)
  vi.mocked(messagingApi.sendMessage).mockResolvedValue({ id: 'm1' } as never)
})
afterEach(resetClient)

const open = (onClose = vi.fn()) => renderWithClient(<DocDialog open onClose={onClose} workspaceId="ws" channelId="ch" />)

describe('DocDialog', () => {
  it('needs a title', async () => {
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Tạo tài liệu' }))
    expect(documentApi.create).not.toHaveBeenCalled()
    expect(await screen.findByText('Nhập tên tài liệu.')).toBeInTheDocument()
  })

  it("creates the document in the channel's folder, then posts it as a document message", async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(documentApi.create).mockResolvedValue(textDoc())
    open(onClose)
    await user.type(screen.getByLabelText('Tên tài liệu'), '  Biên bản họp  ')
    await user.click(screen.getByRole('button', { name: 'Tạo tài liệu' }))
    await waitFor(() => expect(documentApi.create).toHaveBeenCalledWith('ws', { title: 'Biên bản họp', folder_id: FOLDER }))
    await waitFor(() => expect(messagingApi.sendMessage).toHaveBeenCalled())
    expect(vi.mocked(messagingApi.sendMessage).mock.calls[0]![0]).toBe('ch')
    expect(vi.mocked(messagingApi.sendMessage).mock.calls[0]![1]).toMatchObject({
      content: '', linked_entity_type: 'document', linked_entity_id: DOC_ID,
    })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('shows the folder by name, not by id', async () => {
    open()
    expect(await screen.findByText('Biên bản')).toBeInTheDocument()
  })

  it('does not create a second document when only the post failed and the user retries', async () => {
    const user = userEvent.setup()
    vi.mocked(documentApi.create).mockResolvedValue(textDoc())
    vi.mocked(messagingApi.sendMessage).mockRejectedValueOnce(new ApiError('boom', 500)).mockResolvedValue({ id: 'm1' } as never)
    open()
    await user.type(screen.getByLabelText('Tên tài liệu'), 'Biên bản')
    await user.click(screen.getByRole('button', { name: 'Tạo tài liệu' }))
    expect(await screen.findByText(/Đã tạo tài liệu nhưng chưa gửi được vào nhóm/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Gửi lại' }))
    await waitFor(() => expect(messagingApi.sendMessage).toHaveBeenCalledTimes(2))
    expect(documentApi.create).toHaveBeenCalledTimes(1)
  })

  it('keeps the dialog open and explains when the document cannot be created', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(documentApi.create).mockRejectedValue(new ApiError('forbidden', 403))
    open(onClose)
    await user.type(screen.getByLabelText('Tên tài liệu'), 'Biên bản')
    await user.click(screen.getByRole('button', { name: 'Tạo tài liệu' }))
    expect(await screen.findByText(/Bạn chưa có quyền tạo tài liệu/)).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
    expect(messagingApi.sendMessage).not.toHaveBeenCalled()
  })
})
