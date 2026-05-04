import type { ReactNode } from 'react'

type BadgeVariant = 'primary' | 'success' | 'warning' | 'danger' | 'info' | 'neutral'

interface BadgeProps {
  variant?: BadgeVariant
  children: ReactNode
  className?: string
}

const variantStyles: Record<BadgeVariant, string> = {
  primary: 'bg-primary-fixed text-primary',
  success: 'bg-success-bg text-success',
  warning: 'bg-warning-bg text-warning',
  danger: 'bg-danger-bg text-danger',
  info: 'bg-info-bg text-info',
  neutral: 'bg-surface-container-high text-on-surface',
}

/** Semantic badge with Material 3 functional color pairs. */
export function Badge({ variant = 'primary', children, className = '' }: BadgeProps) {
  return (
    <span className={`inline-flex items-center px-2 py-1 text-micro rounded-full
      ${variantStyles[variant]} ${className}`}>
      {children}
    </span>
  )
}
