import { type InputHTMLAttributes, forwardRef } from 'react'

type InputVariant = 'default' | 'search'
type InputRadius = 'md' | 'lg'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string
  helpText?: string
  variant?: InputVariant
  radius?: InputRadius
}

/**
 * Bo góc đi qua prop chứ không qua `className`: `rounded-md` nướng trong chuỗi
 * sẽ hoà độ đặc hiệu với `rounded-lg` truyền vào và thắng theo thứ tự stylesheet,
 * làm ô nhập sụp từ 12px về 8px mà không gì báo. Đã xảy ra 3 lần ở chặng 1.
 * Xem spec 2026-08-01 §7.5.
 *
 * `search` là pill nên bỏ qua `radius` — Stitch §Inputs quy định như vậy.
 */
const radiusStyles: Record<InputRadius, string> = {
  md: 'rounded-md',
  lg: 'rounded-lg',
}

const variantStyles: Record<InputVariant, string> = {
  default: 'bg-surface-container-lowest',
  search: 'rounded-full bg-surface-container-low',
}

/** Recessed input — surface-container-lowest background for visual depth on elevated surfaces. */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = '', error, helpText, variant = 'default', radius = 'md', ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <input
        ref={ref}
        className={`w-full px-3 py-2 text-small border
          text-on-surface placeholder:text-outline
          transition-colors duration-fast focus-ring
          ${variant === 'search' ? '' : radiusStyles[radius]}
          ${variantStyles[variant]}
          ${error
            ? 'border-danger focus:border-danger'
            : 'border-outline-variant focus:border-primary'
          } ${className}`}
        {...props}
      />
      {error && <span className="text-micro text-danger">{error}</span>}
      {helpText && !error && <span className="text-micro text-outline">{helpText}</span>}
    </div>
  ),
)

Input.displayName = 'Input'
