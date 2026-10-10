import { useEffect, useRef, useState } from 'react'
import { Button, TextField } from '../primitives'
import { Dialog } from '../composites/Dialog'

const NAME_MAX = 255

interface NameDialogProps {
  open: boolean
  onClose: () => void
  /** Dialog title, also the verb: "Thư mục mới", "Đổi tên". */
  title: string
  /** Label of the confirm button. */
  submitLabel: string
  /** Shown when the name is empty, e.g. "Đặt tên cho thư mục". */
  emptyMessage: string
  initialName?: string
  pending: boolean
  onSubmit: (name: string) => void
}

/**
 * Asks for one name: a new folder, or the new name of an item. A small dialog
 * instead of the browser's `prompt()`, so it is in the app's language, keyboard
 * and focus rules, and can say what is wrong with the name.
 */
export function NameDialog({
  open, onClose, title, submitLabel, emptyMessage, initialName = '', pending, onSubmit,
}: NameDialogProps) {
  const [name, setName] = useState(initialName)
  const [error, setError] = useState<string | undefined>()
  const inputRef = useRef<HTMLInputElement>(null)

  // Start from the current name each time the dialog opens, and only then: the
  // item being renamed can change under an open dialog (a realtime update), and
  // that must not throw away what the user has typed.
  const initial = useRef(initialName)
  initial.current = initialName
  useEffect(() => {
    if (open) {
      setName(initial.current)
      setError(undefined)
    }
  }, [open])

  const submit = () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setError(emptyMessage)
      inputRef.current?.focus()
      return
    }
    onSubmit(trimmed)
  }

  return (
    <Dialog
      open={open}
      onClose={pending ? () => {} : onClose}
      title={title}
      initialFocusRef={inputRef}
      onSubmit={submit}
      footer={
        <>
          <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
          <Button type="submit" loading={pending}>{submitLabel}</Button>
        </>
      }
    >
      <TextField
        ref={inputRef}
        label="Tên"
        value={name}
        maxLength={NAME_MAX}
        error={error}
        onChange={(e) => {
          setName(e.target.value)
          if (error) setError(undefined)
        }}
        onFocus={(e) => e.currentTarget.select()}
        autoComplete="off"
      />
    </Dialog>
  )
}
