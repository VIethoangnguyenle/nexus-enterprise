import type { HTMLAttributes, ReactNode } from 'react'

type TextVariant = 'body' | 'caption' | 'label' | 'mono'

interface TextProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: TextVariant
  muted?: boolean
  children: ReactNode
}

const variantStyles: Record<TextVariant, string> = {
  body: 'text-sm leading-relaxed',
  caption: 'text-xs leading-normal',
  label: 'text-xs font-medium uppercase tracking-wider',
  mono: 'text-xs font-mono',
}

/** Typography primitive for inline text. */
export function Text({ variant = 'body', muted = false, className = '', children, ...props }: TextProps) {
  return (
    <span
      className={`${variantStyles[variant]} ${muted ? 'text-text-muted' : 'text-text-secondary'} ${className}`}
      {...props}
    >
      {children}
    </span>
  )
}
