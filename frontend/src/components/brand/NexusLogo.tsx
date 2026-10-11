import { NexusMark } from './NexusMark'

interface NexusLogoProps {
  /** Edge of the mark in px; the name scales with it. */
  size?: number
  /** A line under the name, e.g. what the product is. */
  subtitle?: string
  assemble?: boolean
  className?: string
}

/**
 * The lockup: the mark, then "Nexus" in the display face (700, tracking -0.01em).
 * The name is the text alternative, so the mark itself is decorative here.
 */
export function NexusLogo({ size = 32, subtitle, assemble = false, className = '' }: NexusLogoProps) {
  return (
    <div className={`flex items-center gap-2.5 ${className}`}>
      <NexusMark size={size} assemble={assemble} decorative />
      <div>
        <div className="font-display font-bold text-lg leading-tight tracking-tight text-ink">Nexus</div>
        {subtitle && <div className="text-xs text-ink-muted">{subtitle}</div>}
      </div>
    </div>
  )
}
