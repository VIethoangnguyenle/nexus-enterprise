type SpinnerSize = 'sm' | 'md' | 'lg'

interface SpinnerProps {
  size?: SpinnerSize
  className?: string
}

const sizeStyles: Record<SpinnerSize, string> = {
  sm: 'w-4 h-4 border-[1.5px]',
  md: 'w-5 h-5 border-2',
  lg: 'w-8 h-8 border-[2.5px]',
}

/** Spinning loading indicator. Uses Tailwind animation. */
export function Spinner({ size = 'md', className = '' }: SpinnerProps) {
  return (
    <div
      className={`inline-block rounded-full border-border border-t-accent animate-spin
        ${sizeStyles[size]} ${className}`}
      role="status"
      aria-label="Loading"
    />
  )
}
