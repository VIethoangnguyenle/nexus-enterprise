import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { LogOut } from 'lucide-react'
import { ApiError } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useLeaveWorkspace } from '../../hooks/useWorkspaces'
import { explain } from '../../lib/errors'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { SettingsGroup, SettingsRow } from './SettingsGroup'

const isLastOwner = (err: unknown) =>
  err instanceof ApiError && err.status === 409 && (err.body as { reason?: string } | undefined)?.reason === 'last_owner'

/**
 * "Rời workspace" (mockup §6): always available, and said plainly: leaving
 * cannot be undone from here, and the last Owner stays. A confirm dialog states
 * the consequence (one sentence and a red button; no retyping of the name).
 * After leaving, the person goes to the workspace picker. `section` is the
 * desktop's danger zone (mockup settings-desktop.html screen 5); `row` is the
 * phone's form, a danger row of Cài đặt workspace (mockup mobile-shell.html §11).
 */
export function LeaveWorkspace({ variant = 'section' }: { variant?: 'section' | 'row' }) {
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
      {variant === 'row' ? (
        <SettingsGroup>
          <SettingsRow
            icon={<LogOut size={18} strokeWidth={1.75} />}
            label={`Rời ${name}`}
            hint="Chủ sở hữu cuối cùng phải chuyển quyền trước"
            danger
            onPress={() => setOpen(true)}
          />
        </SettingsGroup>
      ) : (
        <div className="flex items-center gap-4 px-4 py-3.5 rounded-lg bg-danger-wash">
          <div className="grid gap-0.5 flex-1 min-w-0">
            <span className="font-semibold text-ink">Rời {name}</span>
            <span className="text-[0.8125rem] text-ink-muted">Bạn mất quyền vào các nhóm và tài liệu của workspace này.</span>
          </div>
          <Button type="button" variant="danger" onClick={() => setOpen(true)}>Rời workspace</Button>
        </div>
      )}
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
