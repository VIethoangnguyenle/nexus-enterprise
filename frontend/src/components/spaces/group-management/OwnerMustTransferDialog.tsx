import { ConfirmDialog } from '../../composites/ConfirmDialog'

/**
 * What "Rời nhóm" opens for the group's owner: a group always has one owner,
 * so they hand the group over first (the server refuses the leave with 409
 * otherwise). With `onTransfer` the dialog leads straight to the transfer.
 */
export function OwnerMustTransferDialog({ open, groupName, onTransfer, onClose }: {
  open: boolean
  groupName: string
  onTransfer?: () => void
  onClose: () => void
}) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        onClose()
        onTransfer?.()
      }}
      title="Chuyển quyền chủ nhóm trước khi rời"
      description={
        <p>
          Bạn là chủ nhóm “{groupName}”. Mỗi nhóm luôn có một chủ nhóm, nên hãy chuyển quyền chủ nhóm cho một thành
          viên khác rồi rời nhóm.
        </p>
      }
      cancelLabel={onTransfer ? 'Huỷ' : 'Đóng'}
      confirmLabel={onTransfer ? 'Chuyển quyền chủ nhóm' : 'Đã hiểu'}
    />
  )
}
