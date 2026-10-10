import { useEffect, useRef, useState } from 'react'
import { useCreateRole } from '../../hooks/useAdmin'
import { statusOf } from '../../lib/errors'
import { Button, TextField, toast } from '../primitives'
import { Dialog } from '../composites/Dialog'

const NAME_MAX = 60

interface CreateRoleDialogProps {
  open: boolean
  onClose: () => void
  workspaceId: string
  /** Called with the new role's id once it exists. */
  onCreated: (roleId: string) => void
}

/**
 * Tạo vai trò: asks for the name. The server turns down a name that reads like
 * one of the platform's own (a prefix such as "Dept_"), and this dialog says so
 * next to the field; any other failure is said in a sentence, not left to a
 * generic toast, because the person is looking at the field.
 */
export function CreateRoleDialog({ open, onClose, workspaceId, onCreated }: CreateRoleDialogProps) {
  const create = useCreateRole(workspaceId)
  const inputRef = useRef<HTMLInputElement>(null)
  const [name, setName] = useState('')
  const [error, setError] = useState<string | undefined>()

  useEffect(() => {
    if (!open) return
    setName('')
    setError(undefined)
    create.reset()
    // The reset belongs to opening only, so `create` is deliberately not a dependency.
  }, [open])

  const submit = () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Đặt tên cho vai trò.')
      inputRef.current?.focus()
      return
    }
    create.mutate(trimmed, {
      onSuccess: (role) => {
        toast(`Đã tạo vai trò ${trimmed}`)
        onClose()
        onCreated(role.id)
      },
      onError: (err) => {
        const status = statusOf(err)
        setError(
          status === 400 ? 'Tên này không dùng được. Đừng bắt đầu hoặc kết thúc bằng tên có sẵn của hệ thống (như Dept_, Role_, _Owners).'
          : status === 403 ? 'Bạn chưa có quyền tạo vai trò.'
          : 'Chưa tạo được vai trò. Kiểm tra kết nối rồi thử lại.',
        )
        inputRef.current?.focus()
      },
    })
  }

  return (
    <Dialog
      open={open}
      onClose={create.isPending ? () => {} : onClose}
      title="Tạo vai trò"
      initialFocusRef={inputRef}
      onSubmit={submit}
      footer={
        <>
          <Button type="button" variant="soft" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
          <Button type="submit" loading={create.isPending}>Tạo vai trò</Button>
        </>
      }
    >
      <TextField
        ref={inputRef}
        label="Tên vai trò"
        placeholder="Ví dụ: Kế toán"
        value={name}
        maxLength={NAME_MAX}
        error={error}
        autoComplete="off"
        onChange={(e) => {
          setName(e.target.value)
          if (error) setError(undefined)
        }}
      />
      <p className="m-0 text-sm text-ink-muted">Sau khi tạo, chọn quyền cho vai trò rồi gán cho từng người.</p>
    </Dialog>
  )
}
