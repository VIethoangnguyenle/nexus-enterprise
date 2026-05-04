import { type ButtonHTMLAttributes, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'success' | 'outline' | 'error'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary-hover',
  secondary: 'bg-surface-container text-on-surface border border-outline-variant hover:bg-surface-container-high',
  outline: 'bg-transparent text-on-surface-variant border border-outline-variant hover:bg-surface-container-high hover:text-on-surface',
  danger: 'bg-danger-bg text-danger border border-danger/20 hover:bg-danger/15',
  error: 'bg-error text-on-error hover:bg-error/90',
  success: 'bg-success-bg text-success border border-success/20 hover:bg-success/15',
  ghost: 'bg-transparent text-on-surface-variant hover:text-on-surface hover:bg-surface-container',
}

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-2 py-1 text-caption-ui rounded-sm',
  md: 'px-3 py-1 text-small-ui rounded-md',
  lg: 'px-4 py-2 text-body-ui rounded-md',
}

/** Flat button primitive — Material 3 surface tokens, no gradients or glow effects. */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, className = '', children, disabled, ...props }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-2
        transition-colors duration-fast cursor-pointer border-none
        disabled:opacity-40 disabled:cursor-not-allowed focus-ring
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {loading && (
        <svg className="animate-spin -ml-1 mr-1 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      )}
      {children}
    </button>
  ),
)

Button.displayName = 'Button'
