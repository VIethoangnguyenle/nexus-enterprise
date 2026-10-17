import type { ReactNode } from 'react'
import { CircleAlert, Inbox, Search, Users } from 'lucide-react'
import { Button, Pressable } from '../primitives'

type Shape = 'dept' | 'person'

/**
 * The rows of a list that is loading, shaped like the real ones: a square for a department, a circle for
 * a person, and two lines of text. No spinner (DESIGN.md "Danh bạ").
 */
export function RowSkeletons({ shapes = ['dept', 'dept', 'person', 'person', 'person', 'person'] }: { shapes?: Shape[] }) {
  const widths = [60, 45, 52, 38, 56, 42]
  return (
    <div role="status" aria-busy="true" className="grid gap-0.5">
      <span className="sr-only">Đang tải…</span>
      {shapes.map((shape, i) => {
        const w = widths[i % widths.length]!
        return (
          <div key={i} aria-hidden="true" className="flex items-center gap-3 min-h-13 px-2 py-2">
            <div data-skeleton-mark={shape} className={`skeleton w-8 h-8 shrink-0 ${shape === 'dept' ? 'rounded-md' : 'rounded-full'}`} />
            <div className="grid flex-1 gap-1.5">
              <div className="skeleton h-3.5 rounded-sm" style={{ width: `${w}%` }} />
              <div className="skeleton h-2.5 rounded-sm" style={{ width: `${Math.round(w / 2)}%` }} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

const ICONS = { inbox: Inbox, search: Search, users: Users, alert: CircleAlert } as const

interface StateCardProps {
  icon?: keyof typeof ICONS
  /** `error` is announced and drawn in the danger tone. */
  tone?: 'info' | 'error'
  title: string
  text?: string
  action?: { label: string; onPress: () => void; icon?: ReactNode }
}

/** An empty or failed list (mockup §5): an icon in a 40 square, what happened in one sentence, one action. */
export function StateCard({ icon, tone = 'info', title, text, action }: StateCardProps) {
  const Icon = ICONS[icon ?? (tone === 'error' ? 'alert' : 'inbox')]
  return (
    <div role={tone === 'error' ? 'alert' : undefined} className="grid justify-items-center gap-2 px-4 py-12 text-center">
      <span
        aria-hidden="true"
        className={`grid place-items-center w-10 h-10 rounded-lg ${tone === 'error' ? 'bg-danger-wash text-danger' : 'bg-sunk text-ink-muted'}`}
      >
        <Icon size={24} strokeWidth={1.75} />
      </span>
      <b className="text-ink max-w-[36ch] text-balance">{title}</b>
      {text && <p className="m-0 max-w-[36ch] text-ink-muted">{text}</p>}
      {action && (
        <Button variant="soft" size="sm" onClick={action.onPress}>
          {action.icon}
          {action.label}
        </Button>
      )}
    </div>
  )
}

/** A source that failed while the others still work: one line, Thử lại beside it. */
export function InlineAlert({ text, onRetry }: { text: string; onRetry: () => void }) {
  return (
    <div role="alert" className="flex items-start gap-2 px-2.5 py-2 rounded-lg bg-danger-wash text-small text-ink">
      <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" className="mt-0.5 shrink-0 text-danger" />
      <span>
        {text}{' '}
        <Pressable onClick={onRetry} className="p-0 border-none bg-transparent font-semibold text-danger cursor-pointer focus-ring rounded-sm">
          Thử lại
        </Pressable>
      </span>
    </div>
  )
}
