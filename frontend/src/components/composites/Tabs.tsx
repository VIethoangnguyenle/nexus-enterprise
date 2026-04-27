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

/** Horizontal tab bar with active state indicator. */
export function Tabs({ tabs, activeId, onChange, className = '' }: TabsProps) {
  return (
    <div
      className={`flex gap-0.5 bg-bg-glass border border-border rounded-[var(--radius-md)]
        p-1 ${className}`}
      role="tablist"
    >
      {tabs.map(tab => (
        <button
          key={tab.id}
          role="tab"
          aria-selected={tab.id === activeId}
          onClick={() => onChange(tab.id)}
          className={`flex items-center gap-2 px-4 py-2 rounded-[var(--radius-sm)]
            text-sm font-medium transition-all duration-200 cursor-pointer border-none
            ${tab.id === activeId
              ? 'bg-accent text-white shadow-[0_2px_8px_var(--color-accent-glow)]'
              : 'bg-transparent text-text-secondary hover:text-text-primary'
            }`}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </div>
  )
}
