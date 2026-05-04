interface AvatarProps {
  name: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const sizeStyles = {
  sm: 'w-6 h-6 text-micro',
  md: 'w-8 h-8 text-caption-ui',
  lg: 'w-10 h-10 text-small-ui',
}

/** Circular avatar with initial letter using Material 3 secondary-container for contrast. */
export function Avatar({ name, size = 'md', className = '' }: AvatarProps) {
  const initial = name.charAt(0).toUpperCase()
  return (
    <div
      className={`rounded-full bg-secondary-container text-on-secondary-container flex items-center justify-center
        flex-shrink-0 font-medium select-none ${sizeStyles[size]} ${className}`}
      title={name}
    >
      {initial}
    </div>
  )
}
