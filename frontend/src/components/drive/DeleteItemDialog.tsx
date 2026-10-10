import { useRef } from 'react'
import type { DriveItem } from '../../api/drive'
import { ConfirmDialog } from '../composites/ConfirmDialog'

export function DeleteItemDialog({ item, pending, onConfirm, onClose }: {
  item: DriveItem | null
  pending: boolean
  onConfirm: (item: DriveItem) => void
  onClose: () => void
}) {
  // Keep the last item while the dialog fades out, so its text does not blank.
  const last = useRef<DriveItem | null>(null)
  if (item) last.current = item
  const shown = item ?? last.current
  const folder = shown?.item_type === 'folder'
  return (
    <ConfirmDialog
      open={!!item}
      onClose={onClose}
      onConfirm={() => item && onConfirm(item)}
      title={folder ? 'Xoá thư mục' : 'Xoá tệp'}
      description={
        <>
          Xoá <b className="font-semibold text-ink">{shown?.name}</b>?{' '}
          {folder ? 'Mọi thứ bên trong cũng bị xoá. ' : ''}Bạn có thể hoàn tác ngay sau khi xoá.
        </>
      }
      confirmLabel="Xoá"
      confirmVariant="danger"
      loading={pending}
    />
  )
}
