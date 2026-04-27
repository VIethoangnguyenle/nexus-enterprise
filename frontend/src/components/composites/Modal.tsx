import { type ReactNode, useEffect } from 'react'

interface ModalOverlayProps {
  onClose: () => void
  children: ReactNode
  size?: 'sm' | 'md' | 'lg'
}

function ModalOverlay({ onClose, children, size = 'md' }: ModalOverlayProps) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const maxWidths = { sm: 'max-w-sm', md: 'max-w-md', lg: 'max-w-[640px]' }

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center
        bg-black/60 backdrop-blur-sm animate-fade-in"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        className={`bg-bg-secondary border border-border rounded-[var(--radius-lg)]
          p-6 w-[90%] ${maxWidths[size]} max-h-[80vh] overflow-y-auto
          shadow-lg animate-slide-up`}
        role="dialog"
        aria-modal="true"
      >
        {children}
      </div>
    </div>
  )
}

function ModalTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <h2 className={`text-lg font-bold text-text-primary tracking-tight mb-4 ${className}`}>
      {children}
    </h2>
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

/** Compound modal. Usage: <Modal onClose={fn}><Modal.Title>...</Modal.Title><Modal.Body>...</Modal.Body><Modal.Actions>...</Modal.Actions></Modal> */
export const Modal = Object.assign(ModalOverlay, {
  Title: ModalTitle,
  Body: ModalBody,
  Actions: ModalActions,
})
