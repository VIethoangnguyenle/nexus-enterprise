import { useEffect, useRef } from 'react'
import { Pressable } from '../primitives'

export interface ScrollTab {
  id: string
  label: string
}

export interface ScrollTabsProps {
  tabs: ScrollTab[]
  activeTab: string
  onTabChange: (tabId: string) => void
  /** Accessible label for the tablist (screen readers). */
  label?: string
}

/**
 * Horizontal scrollable tab bar with active indicator (DESIGN.md §5).
 * Active tab has a bottom-border indicator + semibold text, and auto-scrolls
 * into view on change. Overflow scrolls horizontally, no wrapping.
 * Used by module headers on mobile for tab/filter switching.
 */
export function ScrollTabs({ tabs, activeTab, onTabChange, label }: ScrollTabsProps) {
  const ref = useRef<HTMLDivElement>(null)

  // Scroll the active tab into view when it changes.
  useEffect(() => {
    const el = ref.current?.querySelector<HTMLElement>('[aria-selected="true"]')
    el?.scrollIntoView?.({ block: 'nearest', inline: 'nearest', behavior: 'smooth' })
  }, [activeTab])

  return (
    <div
      ref={ref}
      role="tablist"
      aria-label={label}
      className="flex gap-0 overflow-x-auto scrollbar-none"
      style={{
        maskImage: 'linear-gradient(to right, black calc(100% - 24px), transparent)',
        WebkitMaskImage: 'linear-gradient(to right, black calc(100% - 24px), transparent)',
      }}
    >
      {tabs.map((tab) => {
        const isActive = tab.id === activeTab
        return (
          <Pressable
            key={tab.id}
            role="tab"
            aria-selected={isActive}
            onClick={() => { if (!isActive) onTabChange(tab.id) }}
            className={`whitespace-nowrap border-b-2 px-4 py-2 text-sm transition-colors
              ${isActive
                ? 'border-accent text-accent font-semibold'
                : 'border-transparent text-ink-muted font-medium hover:text-ink'
              }`}
          >
            {tab.label}
          </Pressable>
        )
      })}
    </div>
  )
}
