import { type ButtonHTMLAttributes, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'success'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: [
    'bg-gradient-to-r from-accent to-[#8b5cf6] text-white',
    'shadow-[0_4px_15px_var(--color-accent-glow)]',
    'hover:shadow-[0_6px_20px_var(--color-accent-glow)] hover:-translate-y-px',
  ].join(' '),
  secondary: [
    'bg-bg-glass text-text-primary border border-border',
    'hover:bg-[rgba(255,255,255,0.08)] hover:border-[rgba(255,255,255,0.15)]',
  ].join(' '),
  danger: 'bg-danger-bg text-danger border border-danger/20 hover:bg-danger/20',
  success: 'bg-success-bg text-success border border-success/20 hover:bg-success/20',
  ghost: 'bg-transparent text-text-secondary hover:text-text-primary hover:bg-bg-hover',
}

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-xs rounded-[var(--radius-sm)]',
  md: 'px-4 py-2 text-sm rounded-[var(--radius-sm)]',
  lg: 'px-5 py-2.5 text-base rounded-[var(--radius-md)]',
}

/** Reusable button with variant and size props. Supports all native button attributes. */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center gap-2 font-medium
        transition-all duration-200 cursor-pointer
        disabled:opacity-40 disabled:cursor-not-allowed disabled:transform-none
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)

Button.displayName = 'Button'
