import { create } from 'zustand'
import { createStore, del, entries, set as idbSet, clear, type UseStore } from 'idb-keyval'
import { driveApi } from '../api/drive'
import { keys } from '../hooks/keys'
import { statusOf } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { onSessionEnd } from '../lib/session-end'
import { useAuthStore } from './auth.store'
import { toast } from '../components/primitives/Toast'

/**
 * Background uploads of Tài liệu (DESIGN.md "Tài liệu", "Nút +"): files go up
 * one at a time while the person keeps working, the progress card follows
 * them, and the queue is kept on the device (IndexedDB, with the file itself)
 * so a reload or an app update picks it up again or says what was lost.
 */

export type UploadStatus = 'queued' | 'uploading' | 'done' | 'failed' | 'lost'

export interface UploadTarget {
  workspaceId: string
  /** The folder it goes into; absent is the drive's top level. */
  parentId?: string
  /** That folder in words, for "Đã tải lên 2 tệp vào Tài chính". */
  folderLabel: string
}

export interface UploadTask extends UploadTarget {
  id: string
  /** Who queued it: a queue left on a shared device is only ever resumed for the same person. */
  owner: string
  name: string
  size: number
  type: string
  status: UploadStatus
  /** Bytes sent of the current attempt. */
  sent: number
  /** When the current attempt started sending (ms), for the time left. */
  startedAt?: number
  /** Why it failed, in words. */
  error?: string
}

interface Stored {
  task: UploadTask
  file?: Blob
}

interface UploadQueueState {
  tasks: UploadTask[]
  /** Adds files to the queue and starts it. */
  enqueue: (files: File[], target: UploadTarget) => void
  /** Sends a failed file again. */
  retry: (id: string) => void
  /** Takes a failed or lost file off the card (and off the device). */
  dismiss: (id: string) => void
  /** Clears what is done from the card. */
  clearDone: () => void
  /** Loads what a previous session left on the device; called once at start. */
  restore: () => Promise<void>
}

let db: UseStore | null = null
const store = () => (db ??= createStore('nexus-upload-queue', 'tasks'))
const hasIdb = () => typeof indexedDB !== 'undefined'

/** The file of each task, held in memory while the page lives. */
const files = new Map<string, Blob>()

/** Writes a task and, the first time, its file. A device that refuses is logged: the upload itself goes on. */
function persist(task: UploadTask, file?: Blob) {
  if (!hasIdb()) return
  idbSet(task.id, { task: { ...task, sent: 0, startedAt: undefined }, file: file ?? files.get(task.id) } satisfies Stored, store())
    .catch((err) => console.warn('[uploads] could not keep the queue on this device', err))
}

function forget(id: string) {
  files.delete(id)
  if (!hasIdb()) return
  del(id, store()).catch((err) => console.warn('[uploads] could not remove a finished upload', err))
}

/** Why an upload failed, as the card says it. */
export function uploadErrorText(err: unknown): string {
  const status = statusOf(err)
  if (status === 413) return 'Hết dung lượng lưu trữ của workspace'
  if (status === 403) return 'Bạn không có quyền tải lên thư mục này'
  if (status === 404) return 'Thư mục không còn nữa'
  return 'Tải lên không thành công'
}

let navigateTo: ((target: UploadTarget) => void) | null = null
/** The screen that can open a folder registers how; the "Xem" of the done toast uses it. */
export function setUploadNavigator(fn: ((target: UploadTarget) => void) | null) {
  navigateTo = fn
}

let running = false
let current: AbortController | null = null
/** Files finished since the queue last ran dry, for one toast at the end. */
let finished: UploadTask[] = []
/** Bumped by a session end: a run that started before it stops. */
let generation = 0

export const useUploadQueue = create<UploadQueueState>()((set, get) => {
  const patch = (id: string, p: Partial<UploadTask>) =>
    set((s) => ({ tasks: s.tasks.map((t) => (t.id === id ? { ...t, ...p } : t)) }))

  const announce = () => {
    const done = finished
    finished = []
    if (done.length === 0) return
    const first = done[0]!
    const oneFolder = done.every((t) => t.workspaceId === first.workspaceId && t.parentId === first.parentId)
    const what = done.length === 1 ? `“${first.name}”` : `${done.length} tệp`
    toast(oneFolder ? `Đã tải lên ${what} vào ${first.folderLabel}` : `Đã tải lên ${what}`, oneFolder && navigateTo
      ? { action: { label: 'Xem', onClick: () => navigateTo?.(first) } }
      : undefined)
  }

  const upload = async (task: UploadTask, file: Blob, gen: number) => {
    current = new AbortController()
    const signal = current.signal
    patch(task.id, { status: 'uploading', sent: 0, startedAt: Date.now(), error: undefined })
    try {
      const created = await driveApi.createFile(task.workspaceId, task.name, task.type || 'application/octet-stream', task.size, task.parentId)
      await driveApi.putWithProgress(created.upload_url, file, (sent) => {
        if (gen === generation) patch(task.id, { sent })
      }, signal)
      await driveApi.confirmFile(created.file_id)
      if (gen !== generation) return
      patch(task.id, { status: 'done', sent: task.size })
      forget(task.id)
      finished.push(task)
      void queryClient.invalidateQueries({ queryKey: keys.drive.all(task.workspaceId) })
    } catch (err) {
      if (gen !== generation) return
      const failed = { ...task, status: 'failed' as const, sent: 0, error: uploadErrorText(err) }
      patch(task.id, failed)
      persist(failed)
    } finally {
      current = null
    }
  }

  const pump = async () => {
    if (running) return
    running = true
    const gen = generation
    try {
      for (;;) {
        if (gen !== generation) return
        const next = get().tasks.find((t) => t.status === 'queued')
        if (!next) break
        const file = files.get(next.id)
        if (!file) {
          patch(next.id, { status: 'lost', error: 'Không còn tệp trên thiết bị, hãy chọn lại' })
          continue
        }
        await upload(next, file, gen)
      }
      announce()
    } finally {
      running = false
      // A sign-out stopped this run; files queued since then still need one.
      if (gen !== generation && get().tasks.some((t) => t.status === 'queued')) void pump()
    }
  }

  return {
    tasks: [],

    enqueue: (picked, target) => {
      if (picked.length === 0) return
      const owner = useAuthStore.getState().user?.id ?? ''
      const added: UploadTask[] = picked.map((f) => ({
        ...target,
        id: crypto.randomUUID(),
        owner,
        name: f.name,
        size: f.size,
        type: f.type,
        status: 'queued',
        sent: 0,
      }))
      added.forEach((t, i) => {
        files.set(t.id, picked[i]!)
        persist(t, picked[i])
      })
      set((s) => ({ tasks: [...s.tasks.filter((t) => t.status !== 'done'), ...added] }))
      void pump()
    },

    retry: (id) => {
      const task = get().tasks.find((t) => t.id === id)
      if (!task || (task.status !== 'failed' && task.status !== 'lost')) return
      if (!files.has(id)) {
        patch(id, { status: 'lost', error: 'Không còn tệp trên thiết bị, hãy chọn lại' })
        return
      }
      const queued = { ...task, status: 'queued' as const, error: undefined, sent: 0 }
      patch(id, queued)
      persist(queued)
      void pump()
    },

    dismiss: (id) => {
      set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) }))
      forget(id)
    },

    clearDone: () => set((s) => ({ tasks: s.tasks.filter((t) => t.status !== 'done') })),

    restore: async () => {
      if (!hasIdb()) return
      let saved: [IDBValidKey, Stored][]
      try {
        saved = await entries<IDBValidKey, Stored>(store())
      } catch (err) {
        console.warn('[uploads] could not read the queue kept on this device', err)
        return
      }
      const known = new Set(get().tasks.map((t) => t.id))
      const me = useAuthStore.getState().user?.id
      const back: UploadTask[] = []
      for (const [, { task, file }] of saved) {
        if (!task?.id || known.has(task.id)) continue
        // Someone else's uploads (a sign-out this page never saw) are dropped, never sent with this session.
        if (!me || task.owner !== me) {
          forget(task.id)
          continue
        }
        if (file) files.set(task.id, file)
        // What was on its way when the page went resumes; what had failed stays failed with its Thử lại.
        const status: UploadStatus = !file ? 'lost' : task.status === 'failed' ? 'failed' : 'queued'
        back.push({
          ...task, status, sent: 0, startedAt: undefined,
          error: status === 'lost' ? 'Tải lên bị gián đoạn, hãy chọn lại tệp' : task.error,
        })
      }
      if (back.length === 0) return
      set((s) => ({ tasks: [...s.tasks, ...back] }))
      void pump()
    },
  }
})

/** Signing out stops the upload in flight and wipes the queue, files included, from the device. */
onSessionEnd(() => {
  generation++
  current?.abort()
  files.clear()
  finished = []
  useUploadQueue.setState({ tasks: [] })
  if (hasIdb()) clear(store()).catch((err) => console.warn('[uploads] could not wipe the queue', err))
})

/** Test seam: forgets module state between tests. */
export function resetUploadQueueForTests() {
  generation++
  running = false
  current = null
  files.clear()
  finished = []
  navigateTo = null
  useUploadQueue.setState({ tasks: [] })
}
