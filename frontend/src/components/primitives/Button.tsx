import { type ButtonHTMLAttributes, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
type ButtonSize = 'sm' | 'md' | 'cta'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary-hover',
  secondary: 'bg-surface-container-lowest text-on-surface border border-outline-variant hover:bg-surface-container-high',
  ghost: 'bg-transparent text-on-surface-variant hover:text-on-surface hover:bg-surface-container',
  danger: 'bg-error text-on-error hover:bg-error/90',
  success: 'bg-success text-on-success hover:bg-success/90',
}

/**
 * Thang size theo `.stitch/DESIGN.md` §Buttons:
 *   md  — Stitch "Primary/Secondary": py-2 px-4, bo 8px, chữ 14/20 w600
 *   cta — Stitch "Full-Width CTA": py-3, bo 12px
 *   sm  — KHÔNG có trong Stitch. Phần mở rộng của dự án cho thanh công cụ và
 *         hàng bảng; 22 chỗ cần tới nó. Xem spec 2026-08-01 §4.2.
 *
 * Bo góc ánh xạ theo pixel Stitch nêu, không theo tên class: thang token của
 * repo là sm 4 / md 8 / lg 12 / xl 16, nên "rounded-lg (8px)" của Stitch là
 * `rounded-md` ở đây. Xem spec §4.1.
 */
const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 rounded-md text-small-ui',
  md: 'px-4 py-2 rounded-md text-body-strong',
  cta: 'w-full py-3 rounded-lg text-body-strong',
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
