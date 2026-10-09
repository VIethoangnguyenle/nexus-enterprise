import { personStyle } from '../../lib/person-hue'

interface SpaceIconProps {
  /** Space name; the icon shows its first letter. */
  name: string
  /** Stable key for the colour (channel id). Hashed, never rendered. */
  hueKey?: string
  size?: 24 | 32 | 40
  className?: string
}

const sizeStyles = {
  24: 'w-6 h-6 rounded-md text-2xs',
  32: 'w-8 h-8 rounded-md text-small',
  40: 'w-10 h-10 rounded-surface text-base',
} as const

/** First letter of a space name, skipping leading punctuation like "#". */
export function spaceLetter(name: string): string {
  const letter = Array.from(name.trim()).find((ch) => /\p{L}|\p{N}/u.test(ch))
  return (letter ?? '?').toLocaleUpperCase('vi')
}

/**
 * A space (nhóm) is a rounded square so it never reads as a person, who is a
 * circle. The fill is a person hue fixed per space.
 */
export function SpaceIcon({ name, hueKey, size = 32, className = '' }: SpaceIconProps) {
  return (
    <span
      className={`inline-grid place-items-center shrink-0 select-none bg-(--p) text-on-person
        font-display font-bold leading-none ${sizeStyles[size]} ${className}`}
      style={personStyle(hueKey || name)}
      aria-hidden="true"
    >
      {spaceLetter(name)}
    </span>
  )
}
