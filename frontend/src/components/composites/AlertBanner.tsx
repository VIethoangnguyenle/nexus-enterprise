import type { ReactNode } from 'react'
import { AlertTriangle, Info, CheckCircle } from 'lucide-react'

type AlertVariant = 'error' | 'warning' | 'info' | 'success'

interface AlertBannerProps {
  /** Visual variant — controls bg/text color and default icon. */
  variant: AlertVariant
  /** Override the default icon for this variant. */
  icon?: ReactNode
  children: ReactNode
  className?: string
}

const variantStyles: Record<AlertVariant, string> = {
  error: 'bg-error-container text-on-error-container',
  warning: 'bg-error-container/70 text-on-error-container',
  info: 'bg-primary-container text-on-primary-container',
  success: 'bg-success-bg text-success',
}

const defaultIcons: Record<AlertVariant, ReactNode> = {
  error: <AlertTriangle size={18} className="flex-shrink-0 mt-0.5" />,
  warning: <AlertTriangle size={18} className="flex-shrink-0 mt-0.5" />,
  info: <Info size={18} className="flex-shrink-0 mt-0.5" />,
  success: <CheckCircle size={18} className="flex-shrink-0 mt-0.5" />,
}

/**
 * Inline alert banner — M3 container tokens.
 *
 * Usage:
 * ```tsx
 * <AlertBanner variant="error">
 *   This action cannot be undone.
 * </AlertBanner>
 * ```
 */
export function AlertBanner({ variant, icon, children, className = '' }: AlertBannerProps) {
  return (
    <div className={`flex items-start gap-3 p-3 rounded-lg ${variantStyles[variant]} ${className}`}>
      {icon ?? defaultIcons[variant]}
      <div className="text-sm leading-snug text-left">{children}</div>
    </div>
  )
}
