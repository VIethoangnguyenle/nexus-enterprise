import type { ReactNode } from 'react'

type BadgeVariant = 'default' | 'primary' | 'success' | 'warning' | 'danger' | 'info'
type BadgeSize = 'sm' | 'md'

interface BadgeProps {
  variant?: BadgeVariant
  size?: BadgeSize
  children: ReactNode
  className?: string
}

const variantStyles: Record<BadgeVariant, string> = {
  default: 'bg-bg-glass border-border text-text-secondary',
  primary: 'bg-accent/10 border-accent/20 text-accent-hover',
  success: 'bg-success-bg border-success/20 text-success',
  warning: 'bg-warning-bg border-warning/20 text-warning',
  danger: 'bg-danger-bg border-danger/20 text-danger',
  info: 'bg-info-bg border-info/20 text-info',
}

const sizeStyles: Record<BadgeSize, string> = {
  sm: 'px-1.5 py-px text-[0.6rem]',
  md: 'px-2.5 py-0.5 text-[0.7rem]',
}

/** Pill-shaped badge for status, type labels, and counts. */
export function Badge({ variant = 'default', size = 'md', children, className = '' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full border font-semibold uppercase tracking-wide whitespace-nowrap
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
    >
      {children}
    </span>
  )
}
