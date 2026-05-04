import { useCallback } from 'react'
import { Trash2 } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import { ConfirmDialog } from '../composites'

interface DeleteConfirmDialogProps {
  /** The item to be deleted — null hides the dialog. */
  item: DriveItem | null
  /** Whether the delete mutation is in progress. */
  isDeleting: boolean
  /** Called when the user confirms deletion. */
  onConfirm: (item: DriveItem) => void
  /** Called when the user cancels or dismisses the dialog. */
  onClose: () => void
}

/**
 * Drive-specific delete confirmation — composes ConfirmDialog.
 *
 * Design source: Nexus Drive - Delete Confirmation (1a692ce8)
 */
export function DeleteConfirmDialog({ item, isDeleting, onConfirm, onClose }: DeleteConfirmDialogProps) {
  const handleConfirm = useCallback(() => {
    if (item) onConfirm(item)
  }, [item, onConfirm])

  return (
    <ConfirmDialog
      open={!!item}
      onClose={onClose}
      onConfirm={handleConfirm}
      title={`Delete ${item?.item_type === 'folder' ? 'Folder' : 'File'}`}
      description={
        <>
          Are you sure you want to permanently delete{' '}
          <span className="font-semibold text-on-surface">{item?.name}</span>?
        </>
      }
      warning="This action cannot be undone. The file will be removed from all shared folders and workspaces immediately."
      icon={<Trash2 size={22} className="text-error" />}
      iconBg="bg-error-container"
      confirmLabel="Delete"
      confirmVariant="error"
      confirmIcon={<Trash2 size={16} />}
      loading={isDeleting}
    />
  )
}
