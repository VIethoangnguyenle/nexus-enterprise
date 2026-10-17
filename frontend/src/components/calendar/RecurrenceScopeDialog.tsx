import { useEffect, useId, useState } from 'react'
import type { EditScope } from '../../api/calendar'
import { Button, Pressable } from '../primitives'
import { Dialog } from '../composites/Dialog'

const CHOICES: { scope: EditScope; label: string }[] = [
  { scope: 'this', label: 'Chỉ sự kiện này' },
  { scope: 'following', label: 'Sự kiện này và các sự kiện sau' },
  { scope: 'all', label: 'Tất cả sự kiện' },
]

interface Props {
  open: boolean
  /** "Dời sự kiện lặp", "Xoá sự kiện lặp": says which change is being made. */
  title: string
  onClose: () => void
  onConfirm: (scope: EditScope) => void
}

/**
 * Which occurrences of a recurring event a change covers (DESIGN.md §6, Lặp lại). Chỉ sự kiện này is the
 * safe default. The editor reuses it for its own save.
 */
export function RecurrenceScopeDialog({ open, title, onClose, onConfirm }: Props) {
  const [scope, setScope] = useState<EditScope>('this')
  const groupId = useId()
  // Each time it opens it offers the narrowest change again.
  useEffect(() => {
    if (open) setScope('this')
  }, [open])

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      width="sm"
      onSubmit={() => onConfirm(scope)}
      footer={
        <>
          <Button type="button" variant="soft" className="rounded-full" onClick={onClose}>Huỷ</Button>
          <Button type="submit" className="rounded-full">OK</Button>
        </>
      }
    >
      <div role="radiogroup" aria-label="Phạm vi thay đổi" id={groupId} className="grid gap-1">
        {CHOICES.map((c) => (
          <Pressable
            key={c.scope}
            role="radio"
            aria-checked={scope === c.scope}
            onClick={() => setScope(c.scope)}
            className="flex items-center gap-3 h-10 px-2 rounded-lg text-sm text-ink hover:bg-hover"
          >
            <span
              aria-hidden="true"
              className={`size-4.5 rounded-full grid place-items-center ring-2 ring-inset
                ${scope === c.scope ? 'ring-accent' : 'ring-ink-muted'}`}
            >
              {scope === c.scope && <span className="size-2.5 rounded-full bg-accent" />}
            </span>
            {c.label}
          </Pressable>
        ))}
      </div>
    </Dialog>
  )
}
