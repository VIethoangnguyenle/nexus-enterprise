import type { ReactNode } from 'react'

type TextVariant = 'body' | 'small' | 'caption' | 'overline' | 'micro'

interface TextProps {
  variant?: TextVariant
  muted?: boolean
  children: ReactNode
  className?: string
}

const variantStyles: Record<TextVariant, string> = {
  body: 'text-body',
  small: 'text-small',
  caption: 'text-caption',
  overline: 'text-overline',
  micro: 'text-micro',
}

/** Text using typography role tokens. Muted variant uses on-surface-variant (secondary). */
export function Text({ variant = 'body', muted, children, className = '' }: TextProps) {
  return (
    <span className={`${variantStyles[variant]} ${muted ? 'text-on-surface-variant' : 'text-on-surface'} ${className}`}>
      {children}
    </span>
  )
}
