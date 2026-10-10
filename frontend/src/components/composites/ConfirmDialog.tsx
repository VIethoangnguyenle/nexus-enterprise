import { useRef, type ReactNode } from 'react'
import { Button } from '../primitives'
import { AlertBanner } from './AlertBanner'
import { Dialog } from './Dialog'

interface ConfirmDialogProps {
  /** Controls visibility — pass false or omit to hide. */
  open: boolean
  /** Close handler (backdrop click, Esc, cancel button). */
  onClose: () => void
  /** Confirm handler. */
  onConfirm: () => void

  /** Dialog title. */
  title: string
  /** What is about to happen — supports ReactNode for inline formatting. */
  description: ReactNode

  /** Optional warning banner below the description. */
  warning?: string
  /** Icon rendered in a colored circle beside the description. */
  icon?: ReactNode
  /** Background class for the icon circle. */
  iconBg?: string

  /** Confirm button label. */
  confirmLabel?: string
  /** Cancel button label. */
  cancelLabel?: string
  /** Confirm button variant — default "primary". */
  confirmVariant?: 'primary' | 'danger'
  /** Icon inside the confirm button. */
  confirmIcon?: ReactNode
  /** Whether the confirm action is in progress. */
  loading?: boolean
}

/**
 * The one confirmation dialog, built on `Dialog`: focus is trapped and
 * returned, Esc closes, and it enters and leaves with the shared motion.
 *
 * A destructive confirm opens with focus on Cancel, so a stray Enter cannot
 * delete anything.
 */
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  warning,
  icon,
  iconBg = 'bg-accent-wash',
  confirmLabel = 'Xác nhận',
  cancelLabel = 'Huỷ',
  confirmVariant = 'primary',
  confirmIcon,
  loading = false,
}: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null)

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      initialFocusRef={confirmVariant === 'danger' ? cancelRef : undefined}
      footer={
        <>
          <Button ref={cancelRef} variant="soft" onClick={onClose} disabled={loading}>
            {cancelLabel}
          </Button>
          <Button variant={confirmVariant} onClick={onConfirm} loading={loading}>
            {confirmIcon}
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="flex items-start gap-3">
        {icon && (
          <span className={`grid place-items-center w-10 h-10 rounded-full shrink-0 ${iconBg}`} aria-hidden="true">
            {icon}
          </span>
        )}
        <div className="text-sm text-ink-muted leading-relaxed">{description}</div>
      </div>
      {warning && <AlertBanner variant="error">{warning}</AlertBanner>}
    </Dialog>
  )
}
