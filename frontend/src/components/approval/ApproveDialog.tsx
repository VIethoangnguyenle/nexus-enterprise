import { useId, useRef, useState } from 'react'
import type { ApprovalRequest } from '../../api/approval'
import { requestTitle } from '../../lib/approval-model'
import { UNKNOWN_PERSON } from '../../lib/people'
import { Dialog } from '../composites/Dialog'
import { Button, Textarea } from '../primitives'

const COMMENT_MAX = 1000

/**
 * "Duyệt đề nghị?": what Duyệt… revealed by a swipe on a phone row opens (DESIGN.md §6 "Vuốt": a swipe only
 * reveals the button, a press opens a confirmation with a comment box). Nothing is approved until Duyệt here.
 * The comment is optional and goes to the requester with the decision.
 */
export function ApproveDialog({ request, pending, onConfirm, onClose }: {
  /** The request to approve; `null` closes the dialog. */
  request: ApprovalRequest | null
  pending: boolean
  onConfirm: (request: ApprovalRequest, comment: string) => void
  onClose: () => void
}) {
  // Keep the last request while the dialog fades out, so its text does not blank.
  const last = useRef<ApprovalRequest | null>(null)
  if (request) last.current = request
  const shown = request ?? last.current

  return (
    <Dialog open={!!request} onClose={pending ? () => {} : onClose} title="Duyệt đề nghị?">
      {shown && <CommentForm key={shown.id} request={shown} pending={pending} onConfirm={onConfirm} onClose={onClose} />}
    </Dialog>
  )
}

function CommentForm({ request, pending, onConfirm, onClose }: {
  request: ApprovalRequest
  pending: boolean
  onConfirm: (request: ApprovalRequest, comment: string) => void
  onClose: () => void
}) {
  const [comment, setComment] = useState('')
  const id = useId()
  const who = request.created_by_name || UNKNOWN_PERSON

  return (
    <form
      className="grid gap-4.5"
      onSubmit={(e) => {
        e.preventDefault()
        onConfirm(request, comment.trim())
      }}
    >
      <p className="m-0 text-sm text-ink-muted leading-relaxed">
        Duyệt “{requestTitle(request)}” của <b className="font-semibold text-ink">{who}</b>.
      </p>
      <div className="grid gap-1.5">
        <label htmlFor={id} className="text-sm font-semibold text-ink">Ý kiến</label>
        <Textarea
          id={id}
          rows={3}
          maxLength={COMMENT_MAX}
          value={comment}
          aria-describedby={`${id}-hint`}
          onChange={(e) => setComment(e.target.value)}
        />
        <span id={`${id}-hint`} className="text-xs text-ink-muted">Không bắt buộc. Người gửi sẽ thấy nội dung này.</span>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
        <Button type="submit" loading={pending}>Duyệt</Button>
      </div>
    </form>
  )
}
