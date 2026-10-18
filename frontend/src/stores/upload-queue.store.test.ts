import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest'
import { clear, createStore, entries, set } from 'idb-keyval'
import { driveApi } from '../api/drive'
import { ApiError } from '../api/client'
import { endSession } from '../lib/session-end'
import { useToastStore } from '../components/primitives/Toast'
import { useAuthStore } from './auth.store'
import { resetUploadQueueForTests, setUploadNavigator, useUploadQueue, type UploadTask } from './upload-queue.store'

const idb = createStore('nexus-upload-queue', 'tasks')
const target = { workspaceId: 'ws-1', parentId: 'f-1', folderLabel: 'Tài chính' }
const file = (name: string, size = 10) => new File(['x'.repeat(size)], name, { type: 'application/pdf' })
const tasks = () => useUploadQueue.getState().tasks
const statusOf = (name: string) => tasks().find((t) => t.name === name)?.status

let createFile: MockInstance<typeof driveApi.createFile>
let put: MockInstance<typeof driveApi.putWithProgress>
let confirm: MockInstance<typeof driveApi.confirmFile>

beforeEach(async () => {
  resetUploadQueueForTests()
  useAuthStore.setState({ user: { id: 'me' } as never })
  await clear(idb)
  useToastStore.getState().clear()
  let n = 0
  createFile = vi.spyOn(driveApi, 'createFile').mockImplementation(async () => {
    n++
    return { file_id: `file-${n}`, upload_url: `https://storage.test/${n}`, object_key: `k${n}` }
  })
  put = vi.spyOn(driveApi, 'putWithProgress').mockImplementation(async (_u, blob, onProgress) => {
    onProgress(blob.size / 2, blob.size)
    onProgress(blob.size, blob.size)
  })
  confirm = vi.spyOn(driveApi, 'confirmFile').mockResolvedValue({})
})
afterEach(() => vi.restoreAllMocks())

describe('upload queue', () => {
  it('uploads files one after another into the chosen folder, then says so once', async () => {
    const view = vi.fn()
    setUploadNavigator(view)
    useUploadQueue.getState().enqueue([file('a.pdf'), file('b.pdf')], target)

    await vi.waitFor(() => expect(tasks().every((t) => t.status === 'done')).toBe(true))
    expect(createFile.mock.calls.map((c) => [c[0], c[1], c[4]])).toEqual([['ws-1', 'a.pdf', 'f-1'], ['ws-1', 'b.pdf', 'f-1']])
    expect(confirm).toHaveBeenCalledTimes(2)

    const toasts = useToastStore.getState().toasts
    expect(toasts.map((t) => t.message)).toEqual(['Đã tải lên 2 tệp vào Tài chính'])
    toasts[0]!.action!.onClick()
    expect(view).toHaveBeenCalledWith(expect.objectContaining({ workspaceId: 'ws-1', parentId: 'f-1' }))
    // Nothing that finished is left on the device.
    expect(await entries(idb)).toEqual([])
  })

  it('reports a failed file with its reason, keeps going, and sends it again on Thử lại', async () => {
    createFile.mockRejectedValueOnce(new ApiError('quota', 413))
    useUploadQueue.getState().enqueue([file('big.pdf'), file('ok.pdf')], target)

    await vi.waitFor(() => expect(statusOf('ok.pdf')).toBe('done'))
    const failed = tasks().find((t) => t.name === 'big.pdf')!
    expect(failed.status).toBe('failed')
    expect(failed.error).toBe('Hết dung lượng lưu trữ của workspace')
    // The failure is kept on the device with its file, so a reload still offers Thử lại.
    await vi.waitFor(async () => expect((await entries(idb)).length).toBe(1))

    useUploadQueue.getState().retry(failed.id)
    await vi.waitFor(() => expect(statusOf('big.pdf')).toBe('done'))
  })

  it('a storage failure is a failure, not a success', async () => {
    put.mockRejectedValueOnce(new Error('Storage upload failed: 500'))
    useUploadQueue.getState().enqueue([file('a.pdf')], target)
    await vi.waitFor(() => expect(statusOf('a.pdf')).toBe('failed'))
    expect(confirm).not.toHaveBeenCalled()
    expect(useToastStore.getState().toasts).toEqual([])
  })

  it('resumes what a previous page left on its way, after a reload', async () => {
    const left: UploadTask = { ...target, owner: 'me', id: 't-1', name: 'sao-ke.pdf', size: 4, type: 'application/pdf', status: 'uploading', sent: 0 }
    await set('t-1', { task: left, file: new Blob(['abcd']) }, idb)

    await useUploadQueue.getState().restore()
    await vi.waitFor(() => expect(statusOf('sao-ke.pdf')).toBe('done'))
    expect(createFile).toHaveBeenCalledWith('ws-1', 'sao-ke.pdf', 'application/pdf', 4, 'f-1')
  })

  it('reports, instead of resuming, an upload whose file the device no longer has', async () => {
    const left: UploadTask = { ...target, owner: 'me', id: 't-2', name: 'mat.pdf', size: 4, type: 'application/pdf', status: 'queued', sent: 0 }
    await set('t-2', { task: left }, idb)

    await useUploadQueue.getState().restore()
    expect(statusOf('mat.pdf')).toBe('lost')
    expect(tasks()[0]!.error).toMatch(/gián đoạn/)
    expect(createFile).not.toHaveBeenCalled()

    useUploadQueue.getState().dismiss('t-2')
    expect(tasks()).toEqual([])
    await vi.waitFor(async () => expect(await entries(idb)).toEqual([]))
  })

  it('never resumes uploads another person left on the device', async () => {
    const theirs: UploadTask = { ...target, owner: 'someone-else', id: 't-3', name: 'cua-ho.pdf', size: 4, type: 'application/pdf', status: 'queued', sent: 0 }
    await set('t-3', { task: theirs, file: new Blob(['abcd']) }, idb)

    await useUploadQueue.getState().restore()
    expect(tasks()).toEqual([])
    expect(createFile).not.toHaveBeenCalled()
    await vi.waitFor(async () => expect(await entries(idb)).toEqual([]))
  })

  it('signing out stops the queue and wipes it from the device', async () => {
    let release: () => void = () => {}
    put.mockImplementationOnce(() => new Promise<void>((resolve) => { release = resolve }))
    useUploadQueue.getState().enqueue([file('a.pdf'), file('b.pdf')], target)
    await vi.waitFor(() => expect(statusOf('a.pdf')).toBe('uploading'))

    endSession()
    expect(tasks()).toEqual([])
    release()
    await vi.waitFor(async () => expect(await entries(idb)).toEqual([]))
    // b.pdf, queued before the sign-out, is never sent for the next person.
    await new Promise((r) => setTimeout(r, 20))
    expect(createFile).toHaveBeenCalledTimes(1)
  })
})
