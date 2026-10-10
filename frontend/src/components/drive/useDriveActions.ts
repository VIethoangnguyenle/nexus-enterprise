import { useCallback, useState } from 'react'
import type { DriveItem } from '../../api/drive'
import {
  useCreateFolder, useMoveItem, useRenameItem, useRestoreItem, useTrashItem, useUploadFile,
} from '../../hooks/useDrive'
import { toast } from '../primitives'

export type Dialog =
  | { kind: 'new-folder' }
  | { kind: 'rename'; item: DriveItem }
  | { kind: 'move'; item: DriveItem }
  | { kind: 'delete'; item: DriveItem }
  | { kind: 'share'; item: DriveItem }

interface Options {
  workspaceId: string
  /** The open folder; uploads and new folders land in it. */
  folderId: string | undefined
  /** The open folder belongs to another workspace: take no uploads. */
  foreign: boolean
  selectedId: string | null
  selectItem: (id: string | null) => void
  /** Called before a change of mine goes out, so its echo is not washed as someone else's. */
  markMine: (id: string) => void
}

/**
 * The write side of Tài liệu: which dialog is open, and the mutations the
 * dialogs and the upload controls trigger. Failures are reported by the shared
 * mutation handler, so each submit only decides whether to close and toast.
 */
export function useDriveActions({ workspaceId, folderId, foreign, selectedId, selectItem, markMine }: Options) {
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const closeDialog = () => setDialog(null)

  const createFolder = useCreateFolder(workspaceId)
  const upload = useUploadFile(workspaceId)
  const rename = useRenameItem(workspaceId)
  const move = useMoveItem(workspaceId)
  const trash = useTrashItem(workspaceId)
  const restore = useRestoreItem(workspaceId)

  const uploadFiles = useCallback(
    async (files: File[]) => {
      if (foreign) return
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
    [upload, folderId, foreign],
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

  const submitMove = async (item: DriveItem, targetFolderId: string) => {
    try {
      markMine(item.id)
      await move.mutateAsync({ itemId: item.id, targetFolderId })
      closeDialog()
      toast(`Đã chuyển “${item.name}”`)
    } catch {
      /* see above */
    }
  }

  const submitDelete = async (item: DriveItem) => {
    try {
      await trash.mutateAsync(item.id)
      closeDialog()
      if (selectedId === item.id) selectItem(null)
      toast(`Đã xoá “${item.name}”`, {
        action: { label: 'Hoàn tác', onClick: () => restore.mutate(item.id) },
      })
    } catch {
      /* see above */
    }
  }

  return {
    dialog, setDialog, closeDialog,
    uploadFiles, uploading: upload.isPending,
    creatingFolder: createFolder.isPending, renaming: rename.isPending, moving: move.isPending, trashing: trash.isPending,
    submitNewFolder, submitRename, submitMove, submitDelete,
  }
}
