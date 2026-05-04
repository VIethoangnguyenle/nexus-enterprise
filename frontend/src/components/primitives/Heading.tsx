import type { ReactNode } from 'react'

type HeadingLevel = 'h1' | 'h2' | 'h3' | 'h4'

interface HeadingProps {
  as?: HeadingLevel
  children: ReactNode
  className?: string
}

const levelStyles: Record<HeadingLevel, string> = {
  h1: 'text-title text-on-surface',
  h2: 'text-title text-on-surface',
  h3: 'text-section text-on-surface',
  h4: 'text-body-strong text-on-surface',
}

/** Heading using Material 3 on-surface token for maximum contrast. */
export function Heading({ as: Tag = 'h2', children, className = '' }: HeadingProps) {
  return <Tag className={`${levelStyles[Tag]} ${className}`}>{children}</Tag>
}
