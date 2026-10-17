import { useState, type CSSProperties } from 'react'
import { hueStyle, personStyle } from '../../lib/person-hue'

/** A picture the auth service (a person's) or the messaging service (a group's) keeps, with no size. */
const STORED_AVATAR = /^\/api\/(?:auth|channels)\/avatars\/[0-9a-f]{32}$/

/**
 * The address of the stored size that fits an avatar of `px` pixels: 64 up to
 * 32 px (sharp at 2×), 192 above it (sharp at 3×), so a list of small avatars
 * never downloads a large picture. Any other address is left as it is.
 */
export function avatarVariant(src: string, px: number): string {
  if (!STORED_AVATAR.test(src)) return src
  return `${src}/${px <= 32 ? 64 : 192}`
}

/** Pixel sizes from DESIGN.md §6. The letter names stay for older call sites. */
type AvatarSize = 20 | 24 | 32 | 36 | 40 | 44 | 52 | 64 | 'sm' | 'md' | 'lg'

interface AvatarProps {
  /** Display name. Initials come from here and nowhere else. */
  name: string
  /**
   * Stable key for the colour (user id). Hashed client-side to one of the
   * eight person hues and never rendered. Defaults to the name.
   */
  hueKey?: string
  /**
   * The colour as an index 0..7 that a server already worked out with the same hash from the user id
   * (the calendar's `PersonRef.hue`). Wins over `hueKey`.
   */
  hue?: number
  size?: AvatarSize
  /** `square` is a group's: corners a quarter of the side, the picture follows. */
  shape?: 'circle' | 'square'
  /** Profile picture, when the person has one. */
  src?: string
  /** Show the green presence dot. */
  online?: boolean
  /** Realtime halo flash: the change came from this person a moment ago. */
  halo?: boolean
  className?: string
}

const px: Record<AvatarSize, 20 | 24 | 32 | 36 | 40 | 44 | 52 | 64> = {
  20: 20, 24: 24, 32: 32, 36: 36, 40: 40, 44: 44, 52: 52, 64: 64, sm: 24, md: 32, lg: 40,
}

const sizeStyles: Record<20 | 24 | 32 | 36 | 40 | 44 | 52 | 64, string> = {
  20: 'w-5 h-5 text-micro',
  24: 'w-6 h-6 text-micro',
  32: 'w-8 h-8 text-xs',
  36: 'w-9 h-9 text-small',
  40: 'w-10 h-10 text-sm',
  44: 'w-11 h-11 text-sm',
  52: 'w-13 h-13 text-16',
  64: 'w-16 h-16 text-lg',
}

const dotStyles: Record<20 | 24 | 32 | 36 | 40 | 44 | 52 | 64, string> = {
  20: 'w-1.5 h-1.5',
  24: 'w-2 h-2',
  32: 'w-2 h-2',
  36: 'w-2.5 h-2.5',
  40: 'w-2.5 h-2.5',
  44: 'w-3 h-3',
  52: 'w-3 h-3',
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
export function Avatar({ name, hueKey, hue, size = 32, shape = 'circle', src, online, halo, className = '' }: AvatarProps) {
  const s = px[size]
  const square = shape === 'square'
  const style: CSSProperties = { ...(hue !== undefined && Number.isInteger(hue) && hue >= 0 && hue <= 7 ? hueStyle(hue + 1) : personStyle(hueKey || name)), ...(square ? { borderRadius: s / 4 } : {}) }
  const round = square ? 'rounded-[inherit]' : 'rounded-full'
  // A picture that fails to load (replaced, removed, or a server that does not
  // serve it) leaves the initials, never an empty circle. A new address tries again.
  const [failed, setFailed] = useState<string | undefined>()
  const shown = src && failed !== src ? avatarVariant(src, s) : undefined
  return (
    <span
      className={`relative inline-grid place-items-center shrink-0 ${square ? '' : 'rounded-full'} select-none
        bg-(--p) text-on-person font-semibold leading-none ${sizeStyles[s]} ${halo ? 'rt-halo' : ''} ${className}`}
      style={style}
      title={name}
      aria-hidden="true"
    >
      {shown
        ? <img src={shown} alt="" className={`w-full h-full ${round} object-cover`} onError={() => setFailed(src)} />
        : initialsOf(name)}
      {online && (
        <span
          className={`absolute -right-px -bottom-px rounded-full bg-success ring-2 ring-base ${dotStyles[s]}`}
        />
      )}
    </span>
  )
}
