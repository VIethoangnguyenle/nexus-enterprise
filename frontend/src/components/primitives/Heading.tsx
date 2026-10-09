import type { ReactNode } from 'react'

type HeadingLevel = 'h1' | 'h2' | 'h3' | 'h4'

/**
 * Tín hiệu display looks (DESIGN.md §3), independent of the semantic level:
 *   page    23/30 700  page title
 *   panel   19/26 700  panel title, channel name
 *   section 17/24 700  side-panel header
 */
type HeadingLook = 'page' | 'panel' | 'section'

interface HeadingProps {
  as?: HeadingLevel
  /** When set, replaces the level's legacy style with a display-font look. */
  look?: HeadingLook
  id?: string
  children: ReactNode
  className?: string
}

const levelStyles: Record<HeadingLevel, string> = {
  h1: 'text-title text-on-surface',
  h2: 'text-title text-on-surface',
  h3: 'text-section text-on-surface',
  h4: 'text-body-strong text-on-surface',
}

const lookStyles: Record<HeadingLook, string> = {
  page: 'font-display text-xl font-bold text-ink',
  panel: 'font-display text-lg font-bold text-ink',
  section: 'font-display text-section font-bold text-ink',
}

/** Heading using Material 3 on-surface token for maximum contrast. */
export function Heading({ as: Tag = 'h2', look, id, children, className = '' }: HeadingProps) {
  return (
    <Tag id={id} className={`m-0 ${look ? lookStyles[look] : levelStyles[Tag]} ${className}`}>
      {children}
    </Tag>
  )
}
