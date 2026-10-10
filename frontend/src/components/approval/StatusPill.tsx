import type { Pill, PillTone } from '../../lib/approval-model'

const TONES: Record<PillTone, string> = {
  wait: 'bg-warning-wash text-warning',
  ok: 'bg-success-wash text-success',
  bad: 'bg-danger-wash text-danger',
  idle: 'bg-sunk text-ink-muted',
}

/**
 * Status as a pill (DESIGN.md §6): a wash of the semantic colour and always a
 * word, so colour is never the only signal.
 */
export function StatusPill({ pill, className = '' }: { pill: Pill; className?: string }) {
  return (
    <span
      className={`inline-flex items-center h-5.5 px-2 rounded-full text-xs font-semibold whitespace-nowrap
        ${TONES[pill.tone]} ${className}`}
    >
      {pill.label}
    </span>
  )
}

/** The count pill on a tab: accent fill, tabular digits. */
export function CountPill({ count }: { count: number }) {
  return (
    <span
      className="inline-flex items-center justify-center min-w-4.5 h-4.5 px-1.5 rounded-full bg-accent
        text-on-accent text-2xs font-semibold tnum"
    >
      {count > 99 ? '99+' : count}
    </span>
  )
}
