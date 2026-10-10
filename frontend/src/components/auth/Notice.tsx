import type { ReactNode } from 'react'
import { AlertCircle, Info, Lock } from 'lucide-react'

type Tone = 'danger' | 'info'

const tones: Record<Tone, string> = {
  danger: 'bg-danger-wash',
  info: 'bg-info-wash',
}
const icons: Record<Tone, string> = { danger: 'text-danger', info: 'text-info' }

/**
 * An inline band for something the person has to read: what went wrong and
 * what to do about it (DESIGN.md §8). `danger` is announced at once
 * (`role="alert"`); `info` is a calm aside and is not.
 */
export function Notice({
  tone = 'danger', icon, children, className = '', live,
}: {
  tone?: Tone
  icon?: 'lock'
  children: ReactNode
  className?: string
  /** Announce it. Defaults to true for `danger`. */
  live?: boolean
}) {
  const Icon = icon === 'lock' ? Lock : tone === 'danger' ? AlertCircle : Info
  const announce = live ?? tone === 'danger'
  return (
    <div
      role={announce ? 'alert' : undefined}
      className={`flex items-start gap-2.5 p-3 rounded-surface text-sm leading-snug text-ink ${tones[tone]} ${className}`}
    >
      <Icon size={16} strokeWidth={1.75} className={`mt-0.5 shrink-0 ${icons[tone]}`} aria-hidden="true" />
      <div className="min-w-0">{children}</div>
    </div>
  )
}
