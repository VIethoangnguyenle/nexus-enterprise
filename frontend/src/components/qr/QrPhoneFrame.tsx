import type { ReactNode } from 'react'
import { X } from 'lucide-react'
import { Heading, IconButton, Text } from '../primitives'

/**
 * One phone screen of sign-in by QR code (design/mockups/qr-login.html §3–§5): a
 * close button, a centred icon tile with a title and a line, whatever the screen
 * shows, and its actions at the bottom in thumb reach.
 */
export function QrPhoneFrame({ onClose, top, icon, tone = 'accent', title, line, children, actions }: {
  /** Shows × top left. */
  onClose?: () => void
  /** Replaces × (the Nexus mark on the signed-out screen). */
  top?: ReactNode
  icon: ReactNode
  tone?: 'accent' | 'success' | 'warning'
  title: string
  line?: ReactNode
  children?: ReactNode
  actions?: ReactNode
}) {
  const tile = {
    accent: 'bg-accent-wash text-accent',
    success: 'bg-success-wash text-success',
    warning: 'bg-warning-wash text-warning',
  }[tone]
  return (
    <main className="min-h-dvh bg-base text-ink grid">
      <div className="w-full max-w-100 mx-auto flex flex-col gap-5 px-4 pt-5 pb-[calc(1.5rem+env(safe-area-inset-bottom,0px))]">
        {top ?? (
          onClose ? (
            <IconButton aria-label="Đóng" size="lg" onClick={onClose} className="-ml-2.5 w-11 h-11">
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          ) : <span className="h-11" aria-hidden="true" />
        )}
        <div className="grid justify-items-center gap-3 text-center pt-6">
          <span aria-hidden="true" className={`grid place-items-center w-16 h-16 rounded-[1.125rem] ${tile}`}>{icon}</span>
          <Heading as="h1" look="panel" className="max-w-[24ch]">{title}</Heading>
          {line && <Text variant="body" muted className="block max-w-[30ch]">{line}</Text>}
        </div>
        {children}
        {actions && <div className="grid gap-2 mt-auto">{actions}</div>}
      </div>
    </main>
  )
}
