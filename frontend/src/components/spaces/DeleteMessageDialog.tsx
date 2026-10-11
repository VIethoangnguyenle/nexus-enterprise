import { useId, useRef } from 'react'
import type { Message } from '../../api/messaging'
import { deleteConsequence } from '../../lib/message-model'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Button } from '../primitives'
import { Dialog } from '../composites/Dialog'

interface DeleteMessageDialogProps {
  /** The message the question is about; nothing shows without one. */
  message: Message | null
  me: string | undefined
  people: PeopleDirectory
  busy: boolean
  onConfirm: () => void
  onCancel: () => void
}

/**
 * "Xoá tin nhắn?" (DESIGN.md "Xoá tin"): always asked, because the message
 * goes for everyone and does not come back. It says whose message it is when
 * that is not yours, that the group's log records it, and that a thread stays.
 * Focus starts on Huỷ, so a stray Enter deletes nothing.
 */
export function DeleteMessageDialog({ message, me, people, busy, onConfirm, onCancel }: DeleteMessageDialogProps) {
  const descriptionId = useId()
  const cancelRef = useRef<HTMLButtonElement>(null)
  const mine = !!message && message.sender_id === me
  const lines = message
    ? deleteConsequence({
        mine,
        senderName: displayName(people, message.sender_id, message.sender_name),
        hasThread: (message.reply_count ?? 0) > 0,
      })
    : []

  return (
    <Dialog
      open={!!message}
      onClose={() => { if (!busy) onCancel() }}
      title="Xoá tin nhắn?"
      role="alertdialog"
      width="sm"
      descriptionId={descriptionId}
      initialFocusRef={cancelRef}
      footer={
        <>
          <Button ref={cancelRef} type="button" variant="soft" onClick={onCancel} disabled={busy}>Huỷ</Button>
          <Button type="button" variant="danger" onClick={onConfirm} loading={busy}>Xoá</Button>
        </>
      }
    >
      <div id={descriptionId} className="grid gap-2 text-sm text-ink-muted leading-relaxed">
        {lines.map((line) => <p key={line}>{line}</p>)}
      </div>
    </Dialog>
  )
}
