import { type ButtonHTMLAttributes, forwardRef } from 'react'

type IconButtonSize = 'sm' | 'md' | 'lg'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize
}

const sizeStyles: Record<IconButtonSize, string> = {
  sm: 'w-7 h-7',
  md: 'w-8 h-8',
  lg: 'w-9 h-9',
}

/** Icon-only button with Material 3 surface tokens. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ size = 'md', className = '', ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center rounded-md
        text-outline hover:text-on-surface hover:bg-surface-container
        transition-colors duration-fast cursor-pointer
        border-none bg-transparent focus-ring
        disabled:opacity-40 disabled:cursor-not-allowed
        ${sizeStyles[size]} ${className}`}
      {...props}
    />
  ),
)

IconButton.displayName = 'IconButton'
