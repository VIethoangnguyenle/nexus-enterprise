import { X } from 'lucide-react'
import { Avatar } from './Avatar'

interface PersonChipProps {
  /** Display name. */
  name: string
  /** Stable key for the avatar colour (user id). Never rendered. */
  hueKey?: string
  /** Muted second part: title or role. */
  role?: string
  avatarUrl?: string
  online?: boolean
  /**
   * `inline` sits in running text and lists. `pill` is a selected token in a
   * picker and carries a remove button when `onRemove` is set.
   */
  variant?: 'inline' | 'pill'
  onRemove?: () => void
  className?: string
}

/** The only way a person is shown: avatar + display name (+ muted role). */
export function PersonChip({
  name, hueKey, role, avatarUrl, online, variant = 'inline', onRemove, className = '',
}: PersonChipProps) {
  if (variant === 'pill') {
    return (
      <span
        className={`inline-flex items-center gap-1.5 h-7 pl-0.5 pr-1 rounded-full bg-sunk
          text-small-ui text-ink max-w-full ${className}`}
      >
        <Avatar name={name} hueKey={hueKey} src={avatarUrl} size={24} />
        <span className="truncate">{name}</span>
        {onRemove && (
          <button
            type="button"
            onClick={onRemove}
            aria-label={`Bỏ ${name}`}
            className="inline-grid place-items-center w-5 h-5 rounded-full border-none bg-transparent
              text-ink-muted cursor-pointer hover:bg-hover hover:text-ink focus-ring
              transition-colors duration-quick"
          >
            <X size={14} strokeWidth={1.75} />
          </button>
        )}
      </span>
    )
  }
  return (
    <span className={`inline-flex items-center gap-2 min-w-0 ${className}`}>
      <Avatar name={name} hueKey={hueKey} src={avatarUrl} online={online} size={20} />
      <span className="truncate font-medium text-ink">{name}</span>
      {role && <span className="truncate text-ink-muted">{role}</span>}
    </span>
  )
}
