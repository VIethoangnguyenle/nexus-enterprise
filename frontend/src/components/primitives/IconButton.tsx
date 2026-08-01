import { type ButtonHTMLAttributes, forwardRef } from 'react'

type IconButtonSize = 'sm' | 'md' | 'lg'
type IconButtonVariant = 'ghost' | 'outlined' | 'filled'
type IconButtonTone = 'default' | 'primary' | 'success' | 'danger'
type IconButtonShape = 'square' | 'round'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize
  variant?: IconButtonVariant
  tone?: IconButtonTone
  /** Mặc định: `filled` là tròn, còn lại vuông bo 8px. */
  shape?: IconButtonShape
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
/**
 * Hình dạng là trục riêng, không gộp vào `variant`. Thanh soạn tin có hàng nút
 * ghost **tròn** (`ChatEditor`), trong khi Stitch §Buttons quy định
 * icon button có viền là 8px — cùng `ghost` nhưng khác hình.
 *
 * Bo góc KHÔNG được nằm ở chuỗi base: `rounded-md` ở đó sẽ hoà độ đặc hiệu với
 * `rounded-full` truyền vào và thắng theo thứ tự stylesheet. `ChatEditor.tsx`
 * đang mắc đúng lỗi này — nó truyền `className="rounded-full"` và các nút render
 * vuông 8px. Cùng cơ chế đã làm variant `secondary` của `Button` mất viền.
 * Xem spec 2026-08-01 §7.5.
 */
const shapeStyles: Record<IconButtonShape, string> = {
  square: 'rounded-md',
  round: 'rounded-full',
}

const variantStyles: Record<IconButtonVariant, string> = {
  ghost: 'bg-transparent border-none',
  outlined: 'bg-transparent border border-outline-variant',
  filled: 'bg-primary text-on-primary border-none shadow-sm hover:bg-primary-hover',
}

const toneStyles: Record<IconButtonTone, string> = {
  default: 'text-outline hover:text-on-surface hover:bg-surface-container',
  primary: 'text-primary hover:bg-primary/10',
  success: 'text-success hover:bg-success-bg',
  danger: 'text-error hover:bg-error/10',
}

/** Icon-only button with Material 3 surface tokens. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ size = 'md', variant = 'ghost', tone = 'default', shape, className = '', ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center
        transition-colors duration-fast cursor-pointer focus-ring
        disabled:opacity-40 disabled:cursor-not-allowed
        ${shapeStyles[shape ?? (variant === 'filled' ? 'round' : 'square')]}
        ${variantStyles[variant]}
        ${variant === 'filled' ? '' : toneStyles[tone]}
        ${sizeStyles[size]} ${className}`}
      {...props}
    />
  ),
)

IconButton.displayName = 'IconButton'
