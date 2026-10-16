import { useState } from 'react'
import { personStyle } from '../../lib/person-hue'
import { avatarVariant } from './Avatar'

interface SpaceIconProps {
  /** Space name; the icon shows its first letter. */
  name: string
  /** Stable key for the colour (channel id). Hashed, never rendered. */
  hueKey?: string
  /** The space's own picture; the letter shows while there is none or it does not load. */
  src?: string
  size?: 24 | 32 | 36 | 40 | 44
  className?: string
}

const sizeStyles = {
  24: 'w-6 h-6 rounded-md text-2xs',
  32: 'w-8 h-8 rounded-md text-small',
  36: 'w-9 h-9 rounded-surface text-16',
  40: 'w-10 h-10 rounded-surface text-16',
  44: 'w-11 h-11 rounded-lg text-md',
} as const

/** First letter of a space name, skipping leading punctuation like "#". */
export function spaceLetter(name: string): string {
  const letter = Array.from(name.trim()).find((ch) => /\p{L}|\p{N}/u.test(ch))
  return (letter ?? '?').toLocaleUpperCase('vi')
}

/**
 * A space (nhóm) is a rounded square so it never reads as a person, who is a
 * circle. The fill is a person hue fixed per space; its picture, when it has
 * one, replaces the letter and follows the same corners.
 */
export function SpaceIcon({ name, hueKey, src, size = 32, className = '' }: SpaceIconProps) {
  // A picture that fails to load leaves the letter, never an empty square. A new address tries again.
  const [failed, setFailed] = useState<string | undefined>()
  const shown = src && failed !== src ? avatarVariant(src, size) : undefined
  return (
    <span
      className={`inline-grid place-items-center shrink-0 select-none bg-(--p) text-on-person
        font-display font-bold leading-none ${sizeStyles[size]} ${className}`}
      style={personStyle(hueKey || name)}
      aria-hidden="true"
    >
      {shown
        ? <img src={shown} alt="" className="w-full h-full rounded-[inherit] object-cover" onError={() => setFailed(src)} />
        : spaceLetter(name)}
    </span>
  )
}
