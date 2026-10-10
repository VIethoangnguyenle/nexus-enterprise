import type { CSSProperties } from 'react'
import { personStyle } from '../../lib/person-hue'

/** Pixel sizes from DESIGN.md §6. The letter names stay for older call sites. */
type AvatarSize = 20 | 24 | 32 | 40 | 64 | 'sm' | 'md' | 'lg'

interface AvatarProps {
  /** Display name. Initials come from here and nowhere else. */
  name: string
  /**
   * Stable key for the colour (user id). Hashed client-side to one of the
   * eight person hues and never rendered. Defaults to the name.
   */
  hueKey?: string
  size?: AvatarSize
  /** Profile picture, when the person has one. */
  src?: string
  /** Show the green presence dot. */
  online?: boolean
  /** Realtime halo flash: the change came from this person a moment ago. */
  halo?: boolean
  className?: string
}

const px: Record<AvatarSize, 20 | 24 | 32 | 40 | 64> = { 20: 20, 24: 24, 32: 32, 40: 40, 64: 64, sm: 24, md: 32, lg: 40 }

const sizeStyles: Record<20 | 24 | 32 | 40 | 64, string> = {
  20: 'w-5 h-5 text-micro',
  24: 'w-6 h-6 text-micro',
  32: 'w-8 h-8 text-xs',
  40: 'w-10 h-10 text-sm',
  64: 'w-16 h-16 text-lg',
}

const dotStyles: Record<20 | 24 | 32 | 40 | 64, string> = {
  20: 'w-1.5 h-1.5',
  24: 'w-2 h-2',
  32: 'w-2 h-2',
  40: 'w-2.5 h-2.5',
  64: 'w-3 h-3',
}

/**
 * Two initials from a display name, the way Vietnamese names are addressed:
 * the last two words ("Lê Thị Hoa" → "TH", "Trần Minh Đức" → "MĐ"). A single
 * word gives its first two letters.
 */
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  const first = (w: string | undefined) => Array.from(w ?? '')[0] ?? ''
  if (words.length === 1) return Array.from(words[0] ?? '').slice(0, 2).join('').toLocaleUpperCase('vi')
  return (first(words[words.length - 2]) + first(words[words.length - 1])).toLocaleUpperCase('vi')
}

/** Round avatar on the person's hue, with presence and the realtime halo. */
export function Avatar({ name, hueKey, size = 32, src, online, halo, className = '' }: AvatarProps) {
  const s = px[size]
  const style: CSSProperties = personStyle(hueKey || name)
  return (
    <span
      className={`relative inline-grid place-items-center shrink-0 rounded-full select-none
        bg-(--p) text-on-person font-semibold leading-none ${sizeStyles[s]} ${halo ? 'rt-halo' : ''} ${className}`}
      style={style}
      title={name}
      aria-hidden="true"
    >
      {src ? <img src={src} alt="" className="w-full h-full rounded-full object-cover" /> : initialsOf(name)}
      {online && (
        <span
          className={`absolute -right-px -bottom-px rounded-full bg-success ring-2 ring-base ${dotStyles[s]}`}
        />
      )}
    </span>
  )
}
