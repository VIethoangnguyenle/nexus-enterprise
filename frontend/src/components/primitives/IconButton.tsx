import { type ButtonHTMLAttributes, forwardRef } from 'react'

type IconButtonSize = 'sm' | 'md' | 'lg'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize
  active?: boolean
  label: string
}

const sizeStyles: Record<IconButtonSize, string> = {
  sm: 'w-7 h-7 text-xs',
  md: 'w-8 h-8 text-sm',
  lg: 'w-10 h-10 text-base',
}

/** Icon-only button with required aria-label. Use for toolbar actions. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ size = 'md', active = false, label, className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      aria-label={label}
      className={`inline-flex items-center justify-center rounded-[var(--radius-sm)]
        border border-border transition-all duration-200 cursor-pointer
        ${active
          ? 'bg-bg-active text-accent-hover border-accent/30'
          : 'bg-transparent text-text-secondary hover:text-text-primary hover:bg-bg-hover'
        }
        disabled:opacity-40 disabled:cursor-not-allowed
        ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)

IconButton.displayName = 'IconButton'
