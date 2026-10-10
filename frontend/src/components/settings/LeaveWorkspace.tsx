import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { ApiError } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useLeaveWorkspace } from '../../hooks/useWorkspaces'
import { explain } from '../../lib/errors'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'

const isLastOwner = (err: unknown) =>
  err instanceof ApiError && err.status === 409 && (err.body as { reason?: string } | undefined)?.reason === 'last_owner'

/**
 * "Rời workspace" (mockup §6): always available, and said plainly: leaving
 * cannot be undone from here, and the last Owner stays. A confirm dialog states
 * the consequence (one sentence and a red button; no retyping of the name).
 * After leaving, the person goes to the workspace picker.
 */
export function LeaveWorkspace() {
  const navigate = useNavigate()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const leave = useLeaveWorkspace(wsId)
  const name = workspaceDisplayName(workspaceName)
  const [open, setOpen] = useState(false)
  const [lastOwner, setLastOwner] = useState(false)

  const close = () => {
    setOpen(false)
    setLastOwner(false) // each opening starts clean
  }

  const confirm = async () => {
    try {
      await leave.mutateAsync()
    } catch (err) {
      if (isLastOwner(err)) setLastOwner(true)
      else toast.error(explain(err, 'rời workspace'))
      return
    }
    setOpen(false)
    toast(`Bạn đã rời ${name}`)
    void navigate({ to: '/workspace-select', search: {} })
  }

  return (
    <>
      <hr className="w-full m-0 border-0 border-t border-line" />
      <div className="flex items-center gap-4">
        <div className="grid flex-1 min-w-0">
          <span className="font-semibold text-ink">Rời {name}</span>
          <span className="text-sm text-ink-muted">
            Bạn mất quyền truy cập tin nhắn, tài liệu và đề nghị của workspace này. Chủ sở hữu cuối cùng không rời được.
          </span>
        </div>
        <Button type="button" variant="danger" onClick={() => setOpen(true)}>Rời workspace</Button>
      </div>
      <ConfirmDialog
        open={open}
        onClose={close}
        onConfirm={() => void confirm()}
        title={`Rời ${name}`}
        description={
          lastOwner ? (
            <span role="alert" className="text-danger font-medium">
              Bạn là chủ sở hữu cuối cùng của workspace này. Chuyển quyền sở hữu cho một người khác ở Quản trị rồi mới rời được.
            </span>
          ) : (
            <>Bạn sẽ mất quyền truy cập mọi thứ trong <b className="font-semibold text-ink">{name}</b> và cần được mời lại để quay về.</>
          )
        }
        confirmLabel="Rời workspace"
        confirmVariant="danger"
        loading={leave.isPending}
      />
    </>
  )
}
