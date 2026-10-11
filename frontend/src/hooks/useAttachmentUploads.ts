import { useState, useCallback, useRef } from 'react'
import { driveApi } from '../api/drive'
import { toast } from '../components/primitives'
import { statusOf } from '../lib/errors'

export interface AttachmentUploadItem {
  key: string
  file: File
  status: 'uploading' | 'done' | 'error'
  progress?: number
  fileId?: string
  width?: number
  height?: number
  error?: string
}

export interface UseAttachmentUploadsOptions {
  workspaceId: string
  channelId: string
}

export function uploadErrorReason(err: unknown): string {
  const status = statusOf(err)
  if (status === 413) return 'Tệp quá lớn hoặc bộ nhớ đã đầy'
  if (status === 403) return 'Không có quyền tải lên'
  if (status === 400) return 'Tệp không hợp lệ'
  if (status && status >= 500) return 'Lỗi máy chủ'
  return 'Mất kết nối, chưa tải lên'
}

export async function measureImage(file: File): Promise<{ width?: number; height?: number }> {
  if (!file.type.startsWith('image/')) return {}
  try {
    if (typeof createImageBitmap === 'function') {
      const bmp = await createImageBitmap(file)
      const res = { width: bmp.width, height: bmp.height }
      bmp.close?.()
      return res
    }
  } catch {
    // fallback to Image
  }
  return new Promise((resolve) => {
    if (typeof Image === 'undefined') {
      resolve({})
      return
    }
    try {
      const url = URL.createObjectURL(file)
      const img = new Image()
      let settled = false
      const done = (w?: number, h?: number) => {
        if (settled) return
        settled = true
        try {
          URL.revokeObjectURL(url)
        } catch {
          // ignore
        }
        resolve(w && h ? { width: w, height: h } : {})
      }
      img.onload = () => done(img.naturalWidth, img.naturalHeight)
      img.onerror = () => done()
      img.src = url
    } catch {
      resolve({})
    }
  })
}

export function useAttachmentUploads({ workspaceId, channelId }: UseAttachmentUploadsOptions) {
  const [items, setItems] = useState<AttachmentUploadItem[]>([])
  const itemsRef = useRef<AttachmentUploadItem[]>([])
  itemsRef.current = items
  const abortControllersRef = useRef(new Map<string, AbortController>())

  const uploadFile = useCallback(
    async (itemKey: string, file: File) => {
      const controller = new AbortController()
      abortControllersRef.current.set(itemKey, controller)

      const updateProgress = (pct: number) => {
        if (controller.signal.aborted) return
        setItems((prev) =>
          prev.map((it) => (it.key === itemKey ? { ...it, progress: pct } : it)),
        )
      }

      try {
        const dimPromise = measureImage(file)
        if (controller.signal.aborted) return

        updateProgress(15)
        const drive = await driveApi.channelDrive(workspaceId, channelId)
        if (controller.signal.aborted) return

        const folder = drive?.items?.find((i) => i.item_type === 'folder')
        updateProgress(35)
        const created = await driveApi.createFile(
          workspaceId,
          file.name,
          file.type || 'application/octet-stream',
          file.size,
          folder?.id,
        )
        if (controller.signal.aborted) return

        updateProgress(65)
        await driveApi.uploadToStorage(created.upload_url, file)
        if (controller.signal.aborted) return

        updateProgress(85)
        await driveApi.confirmFile(created.file_id)
        if (controller.signal.aborted) return

        const dims = await dimPromise
        if (controller.signal.aborted) return

        abortControllersRef.current.delete(itemKey)

        setItems((prev) =>
          prev.map((it) =>
            it.key === itemKey
              ? {
                  ...it,
                  status: 'done',
                  progress: 100,
                  fileId: created.file_id,
                  width: dims.width,
                  height: dims.height,
                  error: undefined,
                }
              : it,
          ),
        )
      } catch (err: unknown) {
        if (controller.signal.aborted) return
        abortControllersRef.current.delete(itemKey)
        toast.error(`Chưa tải lên được “${file.name}”…`)
        setItems((prev) =>
          prev.map((it) =>
            it.key === itemKey
              ? {
                  ...it,
                  status: 'error',
                  progress: 0,
                  error: uploadErrorReason(err),
                }
              : it,
          ),
        )
      }
    },
    [workspaceId, channelId],
  )

  const add = useCallback(
    (files: File[]) => {
      if (!files.length) return
      const currentCount = itemsRef.current.length
      if (currentCount >= 20) {
        toast.error('Tối đa 20 tệp mỗi tin')
        return
      }

      const availableSlots = 20 - currentCount
      const acceptedFiles = files.slice(0, availableSlots)
      if (files.length > availableSlots) {
        toast.error('Tối đa 20 tệp mỗi tin')
      }

      const newItems: AttachmentUploadItem[] = acceptedFiles.map((file) => ({
        key: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
        file,
        status: 'uploading',
        progress: 0,
      }))

      itemsRef.current = [...itemsRef.current, ...newItems]
      setItems((prev) => [...prev, ...newItems])

      for (const item of newItems) {
        void uploadFile(item.key, item.file)
      }
    },
    [uploadFile],
  )

  const remove = useCallback((key: string) => {
    abortControllersRef.current.get(key)?.abort()
    abortControllersRef.current.delete(key)
    itemsRef.current = itemsRef.current.filter((it) => it.key !== key)
    setItems((prev) => prev.filter((it) => it.key !== key))
  }, [])

  const retry = useCallback(
    (key: string) => {
      const target = itemsRef.current.find((it) => it.key === key)
      if (!target) return
      abortControllersRef.current.get(key)?.abort()
      abortControllersRef.current.delete(key)
      setItems((prev) =>
        prev.map((it) => (it.key === key ? { ...it, status: 'uploading', progress: 0, error: undefined } : it)),
      )
      void uploadFile(target.key, target.file)
    },
    [uploadFile],
  )

  const clear = useCallback(() => {
    for (const controller of abortControllersRef.current.values()) {
      controller.abort()
    }
    abortControllersRef.current.clear()
    itemsRef.current = []
    setItems([])
  }, [])

  const ready = items.every((it) => it.status === 'done')

  return {
    items,
    add,
    remove,
    retry,
    clear,
    ready,
  }
}
