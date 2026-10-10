import { useId, useRef, useState } from 'react'
import type { ApprovalRequest } from '../../api/approval'
import { requestTitle } from '../../lib/approval-model'
import { UNKNOWN_PERSON } from '../../lib/people'
import { Dialog } from '../composites/Dialog'
import { Button, Textarea } from '../primitives'

const REASON_MAX = 1000

interface ReturnDialogProps {
  /** The request being returned; `null` closes the dialog. */
  request: ApprovalRequest | null
  pending: boolean
  onConfirm: (request: ApprovalRequest, reason: string) => void
  onClose: () => void
}

/**
 * "Trả lại đề nghị?" (mockup §3): the one modal of the screen, used because a
 * return needs a reason the requester will read. The reason is required; an
 * empty one is refused on the spot and nothing is sent.
 */
export function ReturnDialog({ request, pending, onConfirm, onClose }: ReturnDialogProps) {
  // Keep the last request while the dialog fades out, so its text does not blank.
  const last = useRef<ApprovalRequest | null>(null)
  if (request) last.current = request
  const shown = request ?? last.current

  return (
    <Dialog open={!!request} onClose={pending ? () => {} : onClose} title="Trả lại đề nghị?">
      {/* Keyed by request: another request starts with an empty reason, never the last one's. */}
      {shown && (
        <ReasonForm key={shown.id} request={shown} pending={pending} onConfirm={onConfirm} onClose={onClose} />
      )}
    </Dialog>
  )
}

function ReasonForm({ request, pending, onConfirm, onClose }: {
  request: ApprovalRequest
  pending: boolean
  onConfirm: (request: ApprovalRequest, reason: string) => void
  onClose: () => void
}) {
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | undefined>()
  const id = useId()
  const ref = useRef<HTMLTextAreaElement>(null)
  const who = request.created_by_name || UNKNOWN_PERSON

  const submit = () => {
    const trimmed = reason.trim()
    if (!trimmed) {
      setError('Nhập lý do trả lại để người gửi biết cần bổ sung gì.')
      ref.current?.focus()
      return
    }
    onConfirm(request, trimmed)
  }

  return (
    <form
      className="grid gap-4.5"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      <p className="m-0 text-sm text-ink-muted leading-relaxed">
        <b className="font-semibold text-ink">{who}</b> sẽ thấy lý do của bạn cho “{requestTitle(request)}” và có thể gửi một đề nghị mới.
      </p>
      <div className="grid gap-1.5">
        <label htmlFor={id} className="text-sm font-semibold text-ink">Lý do trả lại</label>
        <Textarea
          ref={ref}
          id={id}
          rows={3}
          maxLength={REASON_MAX}
          value={reason}
          aria-invalid={!!error || undefined}
          aria-describedby={`${id}-hint`}
          onChange={(e) => {
            setReason(e.target.value)
            if (error) setError(undefined)
          }}
        />
        <span id={`${id}-hint`} role={error ? 'alert' : undefined} className={`text-xs ${error ? 'font-medium text-danger' : 'text-ink-muted'}`}>
          {error ?? 'Bắt buộc. Người gửi sẽ thấy nội dung này.'}
        </span>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
        <Button type="submit" variant="danger" loading={pending}>Trả lại</Button>
      </div>
    </form>
  )
}
