import type { ReactNode } from 'react'

export type CardTone = 'accent' | 'success' | 'info' | 'warning'

const TONES: Record<CardTone, string> = {
  accent: 'bg-accent-wash text-accent',
  success: 'bg-success-wash text-success',
  info: 'bg-info-wash text-info',
  warning: 'bg-warning-wash text-warning',
}

interface ChatCardProps {
  tone: CardTone
  /** The 32px square at the head: an icon, or a control (the task's check). */
  icon: ReactNode
  title: ReactNode
  /** "Loại · chi tiết", 12px and muted. */
  sub?: ReactNode
  children?: ReactNode
  /** Strike the title through and mute it (a finished task). */
  done?: boolean
  label?: string
}

/**
 * The frame every in-chat card shares (DESIGN.md "Thẻ (ChatCard)"): a raised
 * surface outside the bubble, at most 420 wide, a toned icon square, a 600
 * title and a muted line under it. No identifier is ever part of it.
 */
export function ChatCard({ tone, icon, title, sub, children, done, label }: ChatCardProps) {
  return (
    <div
      data-card
      role="group"
      aria-label={label}
      className="grid gap-2.5 w-full max-w-105 px-3.5 py-3 rounded-lg bg-raised border border-line"
    >
      <div className="flex items-start gap-2.5 min-w-0">
        <span className={`grid place-items-center shrink-0 w-8 h-8 rounded-md ${TONES[tone]}`}>{icon}</span>
        <span className="grid min-w-0">
          <b className={`font-semibold text-ink break-words ${done ? 'line-through text-ink-muted' : ''}`}>{title}</b>
          {sub && <span className="text-xs text-ink-muted">{sub}</span>}
        </span>
      </div>
      {children}
    </div>
  )
}

/** A card whose object the viewer may not open: the kind and one plain line, nothing of the object. */
export function UnavailableCard({ tone, icon, title, note }: { tone: CardTone; icon: ReactNode; title: string; note?: string }) {
  return (
    <ChatCard tone={tone} icon={icon} title={<span className="text-ink-muted">{title}</span>} sub={note} />
  )
}

export function CardSkeleton() {
  return (
    <div data-card aria-busy="true" className="grid gap-2 w-full max-w-105 px-3.5 py-3 rounded-lg bg-raised border border-line">
      <div className="skeleton h-4 w-2/3 rounded-sm" />
      <div className="skeleton h-3 w-1/2 rounded-sm" />
    </div>
  )
}
