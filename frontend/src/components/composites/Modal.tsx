import { type ReactNode, useEffect } from 'react'
import { X } from 'lucide-react'
import { IconButton } from '../primitives'

type ModalSize = 'sm' | 'md' | 'lg'

interface ModalOverlayProps {
  onClose: () => void
  children: ReactNode
  size?: ModalSize
}

/** Modal overlay — M3 surface tokens, ESC-to-close, backdrop click. */
function ModalOverlay({ onClose, children, size = 'md' }: ModalOverlayProps) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const maxWidths: Record<ModalSize, string> = {
    sm: 'max-w-sm',
    md: 'max-w-md',
    lg: 'max-w-[640px]',
  }

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50 animate-fade-in" />

      {/* Dialog container — M3 surface-container-lowest */}
      <div
        className={`relative z-10 bg-surface-container-lowest rounded-xl
          mx-4 w-full ${maxWidths[size]} max-h-[80vh] overflow-y-auto
          shadow-lg animate-scale-in`}
        role="dialog"
        aria-modal="true"
      >
        {children}
      </div>
    </div>
  )
}

/** Modal header with title and optional close button. */
function ModalHeader({ children, onClose, className = '' }: {
  children: ReactNode
  onClose?: () => void
  className?: string
}) {
  return (
    <div className={`flex items-center justify-between px-6 py-4 border-b border-outline-variant ${className}`}>
      <h3 className="font-h3 text-on-surface">{children}</h3>
      {onClose && (
        <IconButton size="sm" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      )}
    </div>
  )
}

/** Modal title — centered, no close button. Use Modal.Header for header with close. */
function ModalTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <h3 className={`font-h3 text-on-surface mb-4 ${className}`}>
      {children}
    </h3>
  )
}

function ModalBody({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`flex flex-col gap-4 ${className}`}>{children}</div>
}

function ModalActions({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`flex justify-end gap-3 mt-6 ${className}`}>{children}</div>
  )
}

/**
 * Compound modal — M3 tokens, scale-in animation.
 *
 * Usage:
 * ```tsx
 * <Modal onClose={fn}>
 *   <Modal.Header onClose={fn}>Title</Modal.Header>
 *   <Modal.Body>...</Modal.Body>
 *   <Modal.Actions>...</Modal.Actions>
 * </Modal>
 *
 * // Or with centered icon layout:
 * <Modal onClose={fn} size="sm">
 *   <Modal.Title>Delete?</Modal.Title>
 *   <Modal.Body>...</Modal.Body>
 *   <Modal.Actions>...</Modal.Actions>
 * </Modal>
 * ```
 */
export const Modal = Object.assign(ModalOverlay, {
  Header: ModalHeader,
  Title: ModalTitle,
  Body: ModalBody,
  Actions: ModalActions,
})
