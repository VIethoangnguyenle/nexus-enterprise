import type { ReactNode } from 'react'

/**
 * ResponsiveDetailPanel renders a detail panel that:
 * - On mobile/tablet (< lg): covers the full screen as an overlay
 * - On desktop (≥ lg): shows inline as a side panel
 *
 * Replaces the repeated pattern of duplicate mobile overlay + desktop inline rendering.
 */
export function ResponsiveDetailPanel({ children }: { children: ReactNode }) {
  return (
    <>
      {/* Mobile overlay */}
      <div className="fixed inset-0 z-50 bg-surface-container-lowest lg:hidden animate-slide-left">
        {children}
      </div>
      {/* Desktop inline */}
      <div className="hidden lg:block">
        {children}
      </div>
    </>
  )
}
