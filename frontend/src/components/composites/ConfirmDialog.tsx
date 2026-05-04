import type { ReactNode } from 'react'
import { Modal } from './Modal'
import { AlertBanner } from './AlertBanner'
import { Button } from '../primitives'

interface ConfirmDialogProps {
  /** Controls visibility — pass false or omit to hide. */
  open: boolean
  /** Close handler (backdrop click, ESC, Cancel button). */
  onClose: () => void
  /** Confirm handler. */
  onConfirm: () => void

  /** Dialog title. */
  title: string
  /** Description below the title — supports ReactNode for inline formatting. */
  description: ReactNode

  /** Optional warning/error banner below description. */
  warning?: string
  /** Icon rendered in a colored circle above the title. */
  icon?: ReactNode
  /** Background class for the icon circle (e.g. "bg-error-container"). */
  iconBg?: string

  /** Confirm button label — default "Confirm". */
  confirmLabel?: string
  /** Confirm button variant — default "primary". */
  confirmVariant?: 'primary' | 'error'
  /** Icon inside the confirm button. */
  confirmIcon?: ReactNode
  /** Whether the confirm action is in progress. */
  loading?: boolean
}

/**
 * Generic confirmation dialog — compose from Modal + AlertBanner + Button.
 *
 * Reuse for: delete file, leave workspace, revoke access, archive channel, etc.
 *
 * Usage:
 * ```tsx
 * <ConfirmDialog
 *   open={!!target}
 *   onClose={close}
 *   onConfirm={handleDelete}
 *   title="Delete File"
 *   description={<>Delete <strong>{name}</strong>?</>}
 *   warning="This action cannot be undone."
 *   icon={<Trash2 size={22} className="text-error" />}
 *   iconBg="bg-error-container"
 *   confirmLabel="Delete"
 *   confirmVariant="error"
 *   loading={isDeleting}
 * />
 * ```
 */
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  warning,
  icon,
  iconBg = 'bg-primary-container',
  confirmLabel = 'Confirm',
  confirmVariant = 'primary',
  confirmIcon,
  loading = false,
}: ConfirmDialogProps) {
  if (!open) return null

  return (
    <Modal onClose={onClose} size="sm">
      <div
        className="p-6 flex flex-col items-center text-center"
        role="alertdialog"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-desc"
      >
        {/* Icon circle */}
        {icon && (
          <div className={`w-12 h-12 rounded-full flex items-center justify-center mb-4 ${iconBg}`}>
            {icon}
          </div>
        )}

        {/* Title */}
        <Modal.Title className="mb-2">
          <span id="confirm-dialog-title">{title}</span>
        </Modal.Title>

        {/* Description */}
        <p id="confirm-dialog-desc" className="text-body-md text-on-surface-variant mb-4 leading-relaxed">
          {description}
        </p>

        {/* Warning banner */}
        {warning && (
          <div className="w-full mb-6">
            <AlertBanner variant="error">{warning}</AlertBanner>
          </div>
        )}

        {/* Actions */}
        <Modal.Actions className="w-full mt-0">
          <Button variant="outline" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            variant={confirmVariant}
            onClick={onConfirm}
            loading={loading}
          >
            {confirmIcon}
            {confirmLabel}
          </Button>
        </Modal.Actions>
      </div>
    </Modal>
  )
}
