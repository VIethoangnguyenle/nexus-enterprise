import { useRef, useState } from 'react'
import type { Department } from '../../api/admin'
import { Button, TextField } from '../primitives'
import { Dialog } from '../composites/Dialog'
import { DepartmentPicker } from './DepartmentPicker'

const NAME_MAX = 120

interface DepartmentDialogProps {
  open: boolean
  onClose: () => void
  departments: Department[]
  /** The department the new one starts under; empty for a root. */
  initialParent: string
  pending: boolean
  onSubmit: (name: string, parentId: string) => void
}

/**
 * Names a new department and says where it sits. The parent is chosen from the
 * tree by name. Mounted fresh each time it opens (the caller keys it), so it
 * never starts from the last department's answers.
 */
export function DepartmentDialog({ open, onClose, departments, initialParent, pending, onSubmit }: DepartmentDialogProps) {
  const [name, setName] = useState('')
  const [parent, setParent] = useState(initialParent)
  const [error, setError] = useState<string | undefined>()
  const inputRef = useRef<HTMLInputElement>(null)

  const submit = () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Đặt tên cho phòng ban.')
      inputRef.current?.focus()
      return
    }
    onSubmit(trimmed, parent)
  }

  return (
    <Dialog
      open={open}
      onClose={pending ? () => {} : onClose}
      title="Phòng ban mới"
      initialFocusRef={inputRef}
      onSubmit={submit}
      footer={
        <>
          <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
          <Button type="submit" loading={pending}>Tạo phòng ban</Button>
        </>
      }
    >
      <TextField
        ref={inputRef}
        label="Tên phòng ban"
        value={name}
        maxLength={NAME_MAX}
        error={error}
        autoComplete="off"
        onChange={(e) => {
          setName(e.target.value)
          if (error) setError(undefined)
        }}
      />
      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">Thuộc phòng</span>
        <DepartmentPicker
          label="Thuộc phòng"
          departments={departments}
          value={parent}
          onChange={setParent}
          noneLabel="Không có (phòng gốc)"
        />
      </div>
    </Dialog>
  )
}
