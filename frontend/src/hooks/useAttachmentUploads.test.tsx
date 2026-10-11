import { renderHook, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAttachmentUploads, uploadErrorReason } from './useAttachmentUploads'
import { driveApi } from '../api/drive'
import { toast } from '../components/primitives'
import { ApiError } from '../api/client'

vi.mock('../api/drive', () => ({
  driveApi: {
    channelDrive: vi.fn(),
    createFile: vi.fn(),
    uploadToStorage: vi.fn(),
    confirmFile: vi.fn(),
    getDownloadUrl: vi.fn(),
  },
}))

globalThis.createImageBitmap = vi.fn().mockImplementation(async () => ({
  width: 1280,
  height: 720,
  close: vi.fn(),
}))

vi.mock('../components/primitives', async () => {
  const actual = await vi.importActual<any>('../components/primitives')
  return {
    ...actual,
    toast: {
      ...actual.toast,
      error: vi.fn(),
      success: vi.fn(),
    },
  }
})

describe('useAttachmentUploads', () => {
  const wsId = 'ws-test-123'
  const channelId = 'ch-test-456'

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(driveApi.channelDrive).mockResolvedValue({
      items: [{ id: 'folder-root', item_type: 'folder', name: 'Channel Files', workspace_id: wsId, drive_context: 'channel', ngac_node_id: 'node-1', owner_id: 'u-1', status: 'active', created_at: '', updated_at: '' }],
    })
    vi.mocked(driveApi.createFile).mockImplementation(async (_ws, name) => ({
      file_id: `file-id-${name}`,
      upload_url: `https://storage.local/upload/${name}`,
      object_key: `key-${name}`,
    }))
    vi.mocked(driveApi.uploadToStorage).mockResolvedValue(undefined)
    vi.mocked(driveApi.confirmFile).mockResolvedValue({} as any)
  })

  it('add 2 files → both upload through mocked driveApi, ready true after', async () => {
    const { result } = renderHook(() => useAttachmentUploads({ workspaceId: wsId, channelId }))

    const file1 = new File(['content-1'], 'file1.txt', { type: 'text/plain' })
    const file2 = new File(['image-bytes'], 'photo.png', { type: 'image/png' })

    act(() => {
      result.current.add([file1, file2])
    })

    expect(result.current.items).toHaveLength(2)
    expect(result.current.items[0]!.status).toBe('uploading')

    await waitFor(() => {
      expect(result.current.ready).toBe(true)
    })

    expect(result.current.items[0]!.status).toBe('done')
    expect(result.current.items[0]!.fileId).toBe('file-id-file1.txt')
    expect(result.current.items[1]!.status).toBe('done')
    expect(result.current.items[1]!.fileId).toBe('file-id-photo.png')
    expect(result.current.items[1]!.width).toBe(1280)
    expect(result.current.items[1]!.height).toBe(720)
    expect(driveApi.createFile).toHaveBeenCalledTimes(2)
    expect(driveApi.uploadToStorage).toHaveBeenCalledTimes(2)
    expect(driveApi.confirmFile).toHaveBeenCalledTimes(2)
  })

  it('one fails → status error, toast shown with Vietnamese reason, retry succeeds', async () => {
    vi.mocked(driveApi.uploadToStorage).mockRejectedValueOnce(new Error('Network error'))

    const { result } = renderHook(() => useAttachmentUploads({ workspaceId: wsId, channelId }))
    const file = new File(['hello'], 'doc.pdf', { type: 'application/pdf' })

    act(() => {
      result.current.add([file])
    })

    await waitFor(() => {
      expect(result.current.items[0]!.status).toBe('error')
    })

    expect(result.current.ready).toBe(false)
    expect(toast.error).toHaveBeenCalledWith('Chưa tải lên được “doc.pdf”…')
    expect(result.current.items[0]!.error).toBe('Mất kết nối, chưa tải lên')

    const failedKey = result.current.items[0]!.key

    // Retry succeeds
    act(() => {
      result.current.retry(failedKey)
    })

    await waitFor(() => {
      expect(result.current.items[0]!.status).toBe('done')
    })
    expect(result.current.ready).toBe(true)
  })

  it('maps errors to Vietnamese reasons based on HTTP status', () => {
    expect(uploadErrorReason(new ApiError('Payload too large', 413))).toBe('Tệp quá lớn hoặc bộ nhớ đã đầy')
    expect(uploadErrorReason(new ApiError('Forbidden', 403))).toBe('Không có quyền tải lên')
    expect(uploadErrorReason(new ApiError('Bad request', 400))).toBe('Tệp không hợp lệ')
    expect(uploadErrorReason(new ApiError('Server error', 500))).toBe('Lỗi máy chủ')
    expect(uploadErrorReason(new Error('Network drop'))).toBe('Mất kết nối, chưa tải lên')
  })

  it('removing a chip aborts in-flight upload so confirmFile is not called', async () => {
    let finishStorage: () => void = () => {}
    vi.mocked(driveApi.uploadToStorage).mockImplementation(
      () => new Promise<void>((resolve) => { finishStorage = resolve }),
    )

    const { result } = renderHook(() => useAttachmentUploads({ workspaceId: wsId, channelId }))
    const file = new File(['content'], 'in-flight.txt', { type: 'text/plain' })

    act(() => {
      result.current.add([file])
    })

    const key = result.current.items[0]!.key
    expect(result.current.items).toHaveLength(1)

    // Remove while uploadToStorage is pending
    act(() => {
      result.current.remove(key)
    })

    expect(result.current.items).toHaveLength(0)

    // Settle storage
    act(() => {
      finishStorage()
    })

    // confirmFile should NEVER be called since item was removed/aborted
    expect(driveApi.confirmFile).not.toHaveBeenCalled()
  })

  it('21st file refused with toast, and multiple synchronous add calls enforce 20 limit', async () => {
    const { result } = renderHook(() => useAttachmentUploads({ workspaceId: wsId, channelId }))

    const filesBatch1 = Array.from({ length: 15 }, (_, i) => new File(['x'], `file-${i}.txt`, { type: 'text/plain' }))
    const filesBatch2 = Array.from({ length: 10 }, (_, i) => new File(['x'], `file-${i + 15}.txt`, { type: 'text/plain' }))

    act(() => {
      result.current.add(filesBatch1)
      result.current.add(filesBatch2)
    })

    expect(result.current.items).toHaveLength(20)
    expect(toast.error).toHaveBeenCalledWith('Tối đa 20 tệp mỗi tin')
  })
})
