import { useId, useRef, useState } from 'react'
import type { AssetRequest } from '../../api/assets'
import { UNKNOWN_PERSON } from '../../lib/people'
import { Dialog } from '../composites/Dialog'
import { AlertBanner } from '../composites/AlertBanner'
import { Button, Textarea } from '../primitives'

const REASON_MAX = 1000

interface RejectDialogProps {
  /** The request being rejected; `null` closes the dialog. */
  request: AssetRequest | null
  pending: boolean
  /** Why the last attempt failed, in words; shown inside the dialog. */
  failure?: string
  onConfirm: (request: AssetRequest, reason: string) => void
  onClose: () => void
}

/**
 * "Từ chối yêu cầu?" (mockup §3): the modal that asks for the reason the
 * requester will read. The reason is required; an empty one is refused on the
 * spot and nothing is sent.
 */
export function RejectDialog({ request, pending, failure, onConfirm, onClose }: RejectDialogProps) {
  // Keep the last request while the dialog fades out, so its text does not blank.
  const last = useRef<AssetRequest | null>(null)
  if (request) last.current = request
  const shown = request ?? last.current

  return (
    <Dialog open={!!request} onClose={pending ? () => {} : onClose} title="Từ chối yêu cầu?">
      {/* Keyed by request: another request starts with an empty reason, never the last one's. */}
      {shown && (
        <ReasonForm key={shown.id} request={shown} pending={pending} failure={failure} onConfirm={onConfirm} onClose={onClose} />
      )}
    </Dialog>
  )
}

function ReasonForm({ request, pending, failure, onConfirm, onClose }: {
  request: AssetRequest
  pending: boolean
  failure?: string
  onConfirm: (request: AssetRequest, reason: string) => void
  onClose: () => void
}) {
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | undefined>()
  const id = useId()
  const ref = useRef<HTMLTextAreaElement>(null)
  const who = request.requester_name || UNKNOWN_PERSON

  const submit = () => {
    const trimmed = reason.trim()
    if (!trimmed) {
      setError('Nhập lý do từ chối để người gửi biết vì sao.')
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
        <b className="font-semibold text-ink">{who}</b> sẽ nhận thông báo kèm lý do của bạn và có thể gửi yêu cầu mới.
      </p>
      <div className="grid gap-1.5">
        <label htmlFor={id} className="text-sm font-semibold text-ink">Lý do từ chối</label>
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
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
        <Button type="submit" variant="danger" loading={pending}>Từ chối</Button>
      </div>
    </form>
  )
}
