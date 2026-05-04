import type { ReactNode } from 'react'

interface Tab {
  id: string
  label: string
  icon?: ReactNode
}

interface TabsProps {
  tabs: Tab[]
  activeId: string
  onChange: (id: string) => void
  className?: string
}

/** Horizontal tab bar with accent bottom indicator. */
export function Tabs({ tabs, activeId, onChange, className = '' }: TabsProps) {
  return (
    <div
      className={`flex border-b border-outline-variant ${className}`}
      role="tablist"
    >
      {tabs.map(tab => (
        <button
          key={tab.id}
          role="tab"
          aria-selected={tab.id === activeId}
          onClick={() => onChange(tab.id)}
          className={`flex items-center gap-2 px-4 py-2 text-small-ui
            transition-colors duration-fast cursor-pointer border-none
            bg-transparent relative focus-ring
            ${tab.id === activeId
              ? 'text-on-surface font-semibold'
              : 'text-on-surface-variant hover:text-on-surface'
            }`}
        >
          {tab.icon}
          {tab.label}
          {tab.id === activeId && (
            <span className="absolute bottom-0 left-2 right-2 h-0.5 bg-accent rounded-full" />
          )}
        </button>
      ))}
    </div>
  )
}
