import type { AssetPillData, AssetTone } from '../../lib/asset-model'

const TONES: Record<AssetTone, string> = {
  wait: 'bg-warning-wash text-warning',
  ok: 'bg-success-wash text-success',
  bad: 'bg-danger-wash text-danger',
  idle: 'bg-sunk text-ink-muted',
  info: 'bg-info-wash text-info',
  acc: 'bg-accent-wash text-accent',
}

/**
 * A state, status or urgency as a pill (DESIGN.md §6): a wash of the semantic
 * colour and always a word, so colour is never the only signal.
 */
export function AssetPill({ pill, className = '' }: { pill: AssetPillData; className?: string }) {
  return (
    <span
      className={`inline-flex items-center h-5.5 px-2 rounded-full text-xs font-semibold whitespace-nowrap
        ${TONES[pill.tone]} ${className}`}
    >
      {pill.label}
    </span>
  )
}
