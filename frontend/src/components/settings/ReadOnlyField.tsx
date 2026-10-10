import { useId } from 'react'
import { Lock } from 'lucide-react'

/**
 * A value that is shown and cannot be changed here, with the reason beside it
 * (mockup §4): a sunk box with a lock, never a silently disabled input.
 */
export function ReadOnlyField({ label, value, reason }: { label: string; value: string; reason: string }) {
  const id = useId()
  return (
    <div className="grid gap-1.5">
      <span id={id} className="text-sm font-semibold text-ink">{label}</span>
      <div
        aria-labelledby={id}
        aria-describedby={`${id}-why`}
        className="flex items-center gap-2 min-h-10 px-3 rounded-md bg-sunk text-base text-ink-subtle"
      >
        <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
        <span className="min-w-0 truncate">{value || 'Chưa có'}</span>
      </div>
      <span id={`${id}-why`} className="text-xs text-ink-muted">{reason}</span>
    </div>
  )
}
