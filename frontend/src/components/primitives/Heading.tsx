import type { HTMLAttributes, ReactNode } from 'react'

type HeadingLevel = 'h1' | 'h2' | 'h3' | 'h4'

interface HeadingProps extends HTMLAttributes<HTMLHeadingElement> {
  as?: HeadingLevel
  children: ReactNode
}

const levelStyles: Record<HeadingLevel, string> = {
  h1: 'text-2xl font-bold tracking-tight',
  h2: 'text-xl font-bold tracking-tight',
  h3: 'text-base font-semibold',
  h4: 'text-sm font-semibold',
}

/** Heading typography primitive with semantic HTML level. */
export function Heading({ as: Tag = 'h2', className = '', children, ...props }: HeadingProps) {
  return (
    <Tag className={`text-text-primary ${levelStyles[Tag]} ${className}`} {...props}>
      {children}
    </Tag>
  )
}
