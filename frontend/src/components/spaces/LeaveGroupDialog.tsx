import { useState } from 'react'
import { useAuthStore } from '../../stores/auth.store'
import { useLeaveGroup } from '../../hooks/useLeaveGroup'
import { ConfirmDialog } from '../composites/ConfirmDialog'

interface LeaveGroupDialogProps {
  open: boolean
  channelId: string
  workspaceId: string
  groupName: string
  /** The number of members if known (from the members panel); undefined when not loaded. */
  memberCount?: number
  onClose: () => void
  /** Called after the server agrees: the caller should close panels. */
  onLeft?: () => void
}

/**
 * Confirm dialog for leaving a group (design/mockups/leave-group.html §3).
 * Four states: normal, last member warning, loading, error.
 * On phone it renders as a bottom sheet (ConfirmDialog handles that).
 */
export function LeaveGroupDialog({ open, channelId, groupName, memberCount, onClose, onLeft }: LeaveGroupDialogProps) {
  const nodeId = useAuthStore((s) => s.user?.ngac_node_id)
  const { leave, isPending, error, clearError } = useLeaveGroup(channelId)
  const [localError, setLocalError] = useState<string | null>(null)

  const displayError = error || localError

  const handleClose = () => {
    if (isPending) return
    clearError()
    setLocalError(null)
    onClose()
  }

  const handleConfirm = async () => {
    if (!nodeId) {
      setLocalError('Không rời được nhóm. Thử lại.')
      return
    }
    const ok = await leave(nodeId, groupName)
    if (ok) {
      onClose()
      onLeft?.()
    }
  }

  const isLastMember = memberCount !== undefined && memberCount <= 1

  return (
    <ConfirmDialog
      open={open}
      onClose={handleClose}
      onConfirm={() => void handleConfirm()}
      title={`Rời \u201C${groupName}\u201D?`}
      description={
        <>
          <p>Bạn sẽ không còn thấy tin nhắn và tệp của nhóm này. Một thành viên trong nhóm có thể thêm bạn lại.</p>
          {isLastMember && (
            <p className="text-danger font-medium mt-2">
              Bạn là người cuối cùng trong nhóm. Rời đi thì không còn ai đọc được nhóm này.
            </p>
          )}
          {displayError && (
            <p role="alert" className="text-danger text-[0.8125rem] flex items-center gap-1.5 mt-2">
              {displayError}
            </p>
          )}
        </>
      }
      confirmLabel={isPending ? 'Đang rời…' : 'Rời nhóm'}
      confirmVariant="danger"
      loading={isPending}
    />
  )
}
