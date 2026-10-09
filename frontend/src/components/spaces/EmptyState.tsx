import type { ReactNode } from 'react'

/** Empty / error state (DESIGN.md §6): a 24px icon, one sentence, one action. */
export function EmptyState({ icon, text, action, compact }: {
  icon: ReactNode
  text: string
  action?: ReactNode
  compact?: boolean
}) {
  return (
    <div
      className={`grid justify-items-center gap-3 px-6 text-center text-ink-muted ${compact ? 'py-8' : 'py-16'}`}
    >
      <span aria-hidden="true">{icon}</span>
      <span className="text-sm max-w-80">{text}</span>
      {action}
    </div>
  )
}
