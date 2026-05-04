import { ChevronRight } from 'lucide-react'

export interface BreadcrumbItem {
  id: string
  label: string
}

interface BreadcrumbsProps {
  root: string
  items: BreadcrumbItem[]
  onNavigate: (index: number) => void
}

/** Breadcrumb navigation for hierarchical data like Drive folders. */
export function Breadcrumbs({ root, items, onNavigate }: BreadcrumbsProps) {
  return (
    <nav className="flex items-center gap-1 text-sm min-w-0" aria-label="Breadcrumb">
      <button
        onClick={() => onNavigate(-1)}
        className={`bg-transparent border-none p-0 cursor-pointer text-sm transition-colors
          ${items.length === 0
            ? 'text-on-surface font-medium'
            : 'text-primary hover:text-primary hover:underline'}`}
      >
        {root}
      </button>
      {items.map((item, i) => (
        <span key={item.id} className="flex items-center gap-1">
          <ChevronRight size={12} className="text-on-surface-variant flex-shrink-0" />
          <button
            onClick={() => onNavigate(i)}
            className={`bg-transparent border-none p-0 cursor-pointer text-sm transition-colors truncate
              ${i === items.length - 1
                ? 'text-on-surface font-medium'
                : 'text-primary hover:text-primary hover:underline'}`}
          >
            {item.label}
          </button>
        </span>
      ))}
    </nav>
  )
}
