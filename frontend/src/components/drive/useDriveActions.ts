import { useCallback, useState } from 'react'
import type { DriveItem } from '../../api/drive'
import {
  useCopyItem, useCreateFolder, useMoveItem, useRenameItem, useRestoreItem, useToggleStars, useTrashItem, useUploadFile,
} from '../../hooks/useDrive'
import { useUploadQueue } from '../../stores/upload-queue.store'
import { toast } from '../primitives'

export type Dialog =
  | { kind: 'new-folder' }
  | { kind: 'rename'; item: DriveItem }
  /** `items` when several are moved together; `item` is the first, which the folder picker leaves out. */
  | { kind: 'move'; item: DriveItem; items?: DriveItem[] }
  | { kind: 'delete'; item: DriveItem }
  | { kind: 'delete-many'; items: DriveItem[] }
  | { kind: 'share'; item: DriveItem }

interface Options {
  workspaceId: string
  /** The open folder; uploads and new folders land in it. */
  folderId: string | undefined
  /** The open folder in words, for the upload card and its toast. */
  folderLabel: string
  /** The open folder belongs to another workspace: take no uploads. */
  foreign: boolean
  /** Below 1024 uploads run in the background queue; on a desktop they keep the toolbar's spinner. */
  background: boolean
  selectedId: string | null
  selectItem: (id: string | null) => void
  /** Called before a change of mine goes out, so its echo is not washed as someone else's. */
  markMine: (id: string) => void
}

/**
 * The write side of Tài liệu: which dialog is open, and the mutations the
 * dialogs, the sheets, the selection bar and the upload controls trigger.
 * Failures are reported by the shared mutation handler, so each submit only
 * decides whether to close and toast.
 */
export function useDriveActions({ workspaceId, folderId, folderLabel, foreign, background, selectedId, selectItem, markMine }: Options) {
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const closeDialog = () => setDialog(null)

  const createFolder = useCreateFolder(workspaceId)
  const upload = useUploadFile(workspaceId)
  const rename = useRenameItem(workspaceId)
  const move = useMoveItem(workspaceId)
  const trash = useTrashItem(workspaceId)
  const restore = useRestoreItem(workspaceId)
  const copy = useCopyItem(workspaceId)
  const stars = useToggleStars(workspaceId)
  const enqueue = useUploadQueue((s) => s.enqueue)

  const uploadFiles = useCallback(
    async (files: File[]) => {
      if (foreign || files.length === 0) return
      if (background) {
        enqueue(files, { workspaceId, parentId: folderId, folderLabel })
        return
      }
      let done = 0
      for (const file of files) {
        try {
          await upload.mutateAsync({ file, parentId: folderId })
          done++
        } catch {
          // The shared mutation handler has told the user which file failed.
        }
      }
      if (done === 1 && files.length === 1) toast(`Đã tải lên “${files[0]!.name}”`)
      else if (done > 0) toast(`Đã tải lên ${done} tệp`)
    },
    [upload, enqueue, background, workspaceId, folderId, folderLabel, foreign],
  )

  const submitNewFolder = async (name: string) => {
    try {
      await createFolder.mutateAsync({ name, parentId: folderId })
      closeDialog()
      toast(`Đã tạo thư mục “${name}”`)
    } catch {
      // Told by the shared handler; the dialog stays open for another try.
    }
  }

  const submitRename = async (item: DriveItem, name: string) => {
    if (name === item.name) return closeDialog()
    try {
      markMine(item.id)
      await rename.mutateAsync({ itemId: item.id, newName: name })
      closeDialog()
    } catch {
      /* see above */
    }
  }

  /** Moves one item, or every item of a selection, into the folder picked. */
  const submitMove = async (items: DriveItem[], targetFolderId: string) => {
    let done = 0
    for (const item of items) {
      if (item.id === targetFolderId) continue
      try {
        markMine(item.id)
        await move.mutateAsync({ itemId: item.id, targetFolderId })
        done++
      } catch {
        /* see above */
      }
    }
    if (done === 0) return
    closeDialog()
    toast(items.length === 1 ? `Đã chuyển “${items[0]!.name}”` : `Đã chuyển ${done} mục`)
  }

  /** Trashes items, with one Hoàn tác for all of them. */
  const submitDelete = async (items: DriveItem[]) => {
    const gone: DriveItem[] = []
    for (const item of items) {
      try {
        await trash.mutateAsync(item.id)
        gone.push(item)
        if (selectedId === item.id) selectItem(null)
      } catch {
        /* see above */
      }
    }
    if (gone.length === 0) return
    closeDialog()
    toast(gone.length === 1 ? `Đã xoá “${gone[0]!.name}”` : `Đã xoá ${gone.length} mục`, {
      action: { label: 'Hoàn tác', onClick: () => gone.forEach((i) => restore.mutate(i.id)) },
    })
  }

  const duplicate = async (item: DriveItem) => {
    try {
      const made = await copy.mutateAsync(item)
      toast(`Đã tạo “${made.name}”`)
    } catch {
      /* see above */
    }
  }

  const setStars = async (items: DriveItem[], star: boolean) => {
    try {
      await stars.mutateAsync({ items, star })
      const what = items.length === 1 ? `“${items[0]!.name}”` : `${items.length} mục`
      toast(star ? `Đã gắn sao ${what}` : `Đã bỏ sao ${what}`)
    } catch {
      /* see above */
    }
  }

  return {
    dialog, setDialog, closeDialog,
    uploadFiles, uploading: upload.isPending,
    creatingFolder: createFolder.isPending, renaming: rename.isPending, moving: move.isPending, trashing: trash.isPending,
    submitNewFolder, submitRename, submitMove, submitDelete, duplicate, setStars,
  }
}
