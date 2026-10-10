import { useState } from 'react'
import type { TextDocument } from '../../api/documents'
import { Button } from '../primitives'
import { Dialog } from '../composites/Dialog'
import { diffLines, textLines, titleOf, type DiffLine } from './document-model'

interface ConflictDialogProps {
  open: boolean
  /** Close without choosing: nothing is written and the page stays as it is. */
  onClose: () => void
  /** What is on my page. */
  mine: { title: string; content: string }
  /** The document as it now stands on the server. */
  theirs: TextDocument
  onTakeTheirs: () => void
  onKeepMine: () => void
}

function Column({ heading, title, lines }: { heading: string; title: string; lines: DiffLine[] }) {
  return (
    <section aria-label={heading} className="grid content-start gap-1.5 min-w-0">
      <h3 className="m-0 text-label text-ink-muted">{heading}</h3>
      <div className="font-semibold text-ink text-sm break-words">{title}</div>
      <div className="grid gap-1 max-h-64 overflow-y-auto">
        {lines.length === 0 && <span className="text-sm text-ink-muted">Chưa có nội dung.</span>}
        {lines.map((l, i) => (
          <div
            key={i}
            data-changed={l.changed || undefined}
            className={`text-sm break-words px-1.5 py-0.5 rounded-sm ${l.changed ? 'bg-warning-wash text-ink' : 'text-ink-muted'}`}
          >
            {l.text}
          </div>
        ))}
      </div>
    </section>
  )
}

/**
 * Shown when a save was refused because someone saved first. Nothing has been
 * written. The person picks which version to keep, and may first look at the two
 * side by side; closing the dialog leaves everything as it was.
 */
export function ConflictDialog({ open, onClose, mine, theirs, onTakeTheirs, onKeepMine }: ConflictDialogProps) {
  const [comparing, setComparing] = useState(false)

  const close = () => {
    setComparing(false)
    onClose()
  }
  const diff = comparing ? diffLines(textLines(mine.content), textLines(theirs.content ?? '')) : null

  return (
    <Dialog
      open={open}
      onClose={close}
      title="Văn bản đã được sửa ở nơi khác"
      footer={
        <>
          <Button type="button" variant="ghost" onClick={() => setComparing((c) => !c)} aria-pressed={comparing}>
            {comparing ? 'Ẩn so sánh' : 'So sánh'}
          </Button>
          <Button type="button" variant="soft" onClick={() => { setComparing(false); onTakeTheirs() }}>
            Tải lại bản mới nhất
          </Button>
          <Button type="button" variant="danger" onClick={() => { setComparing(false); onKeepMine() }}>
            Giữ bản của tôi
          </Button>
        </>
      }
    >
      <div className="grid gap-3 text-sm text-ink-muted leading-relaxed">
        <p className="m-0">
          Có người vừa lưu một bản mới hơn trong lúc bạn đang sửa, nên bản của bạn chưa được lưu và chưa ghi đè gì.
        </p>
        <ul className="m-0 pl-5 grid gap-1">
          <li><b className="font-semibold text-ink">Tải lại bản mới nhất</b>: bỏ thay đổi chưa lưu của bạn, lấy bản trên máy chủ.</li>
          <li><b className="font-semibold text-ink">Giữ bản của tôi</b>: lưu bản của bạn đè lên bản mới, thay đổi của người kia sẽ mất.</li>
        </ul>
      </div>
      {diff && (
        <div className="grid grid-cols-2 gap-3">
          <Column heading="Bản của bạn" title={titleOf({ title: mine.title })} lines={diff.mine} />
          <Column heading="Bản trên máy chủ" title={titleOf(theirs)} lines={diff.theirs} />
        </div>
      )}
    </Dialog>
  )
}
