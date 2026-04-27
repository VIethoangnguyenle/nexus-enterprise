type AvatarSize = 'sm' | 'md' | 'lg'

interface AvatarProps {
  name: string
  size?: AvatarSize
  className?: string
}

const sizeStyles: Record<AvatarSize, string> = {
  sm: 'w-6 h-6 text-[0.55rem]',
  md: 'w-8 h-8 text-xs',
  lg: 'w-10 h-10 text-sm',
}

/** Deterministic color from string. Returns a hue for HSL background. */
function nameToHue(name: string): number {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return Math.abs(hash) % 360
}

/** Avatar with auto-generated initials and deterministic color from name. */
export function Avatar({ name, size = 'md', className = '' }: AvatarProps) {
  const hue = nameToHue(name)
  const initials = name
    .split(' ')
    .map(w => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  return (
    <div
      className={`rounded-full flex items-center justify-center font-semibold flex-shrink-0
        ${sizeStyles[size]} ${className}`}
      style={{
        background: `linear-gradient(135deg, hsl(${hue}, 60%, 25%), hsl(${hue}, 50%, 35%))`,
        color: `hsl(${hue}, 70%, 75%)`,
      }}
      aria-label={name}
      title={name}
    >
      {initials}
    </div>
  )
}
