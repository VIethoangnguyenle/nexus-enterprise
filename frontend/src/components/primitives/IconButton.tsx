import { type ButtonHTMLAttributes, forwardRef } from 'react'

type IconButtonSize = 'sm' | 'md' | 'lg'
type IconButtonVariant = 'ghost' | 'outlined' | 'filled'
type IconButtonTone = 'default' | 'primary' | 'success' | 'danger'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize
  variant?: IconButtonVariant
  tone?: IconButtonTone
}

const sizeStyles: Record<IconButtonSize, string> = {
  sm: 'w-7 h-7',
  md: 'w-8 h-8',
  lg: 'w-9 h-9',
}

/**
 * `outlined` đến từ Stitch §Buttons ("Icon Button: p-2 rounded-lg border").
 * `filled` là phần mở rộng của dự án cho nút gửi tin nhắn — không có trong Stitch.
 */
const variantStyles: Record<IconButtonVariant, string> = {
  ghost: 'bg-transparent border-none',
  outlined: 'bg-transparent border border-outline-variant',
  filled: 'bg-primary text-on-primary border-none hover:bg-primary-hover',
}

const toneStyles: Record<IconButtonTone, string> = {
  default: 'text-outline hover:text-on-surface hover:bg-surface-container',
  primary: 'text-primary hover:bg-primary/10',
  success: 'text-success hover:bg-success-bg',
  danger: 'text-error hover:bg-error/10',
}

/** Icon-only button with Material 3 surface tokens. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ size = 'md', variant = 'ghost', tone = 'default', className = '', ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center rounded-md
        transition-colors duration-fast cursor-pointer focus-ring
        disabled:opacity-40 disabled:cursor-not-allowed
        ${variantStyles[variant]}
        ${variant === 'filled' ? '' : toneStyles[tone]}
        ${sizeStyles[size]} ${className}`}
      {...props}
    />
  ),
)

IconButton.displayName = 'IconButton'
