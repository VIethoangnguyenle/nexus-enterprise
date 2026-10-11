import { BellOff } from 'lucide-react'

/** The bell-off at the end of a muted row: 14px, quiet. Its words are said by the row (`MutedText`). */
export function MutedMark({ className = '' }: { className?: string }) {
  return <BellOff size={14} strokeWidth={1.75} aria-hidden="true" className={`text-ink-subtle shrink-0 ${className}`} />
}

/** Read by a screen reader after the name of a muted conversation. */
export function MutedText() {
  return <span className="sr-only">đã tắt thông báo</span>
}

/**
 * "@" in the size of a count pill: someone named the reader in a muted
 * conversation (the only thing that conversation still counts).
 */
export function MentionPill({ n, className = '' }: { n: number; className?: string }) {
  return (
    <span
      aria-label={`${n} lần nhắc tên bạn`}
      className={`inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
        text-on-accent text-2xs font-semibold shrink-0 ${className}`}
    >
      @
    </span>
  )
}
