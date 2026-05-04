import { Filter } from 'lucide-react'

export interface ContactFilters {
  department: string
  location: string
  search: string
}

interface ContactsFilterBarProps {
  filters: ContactFilters
  onFiltersChange: (filters: ContactFilters) => void
  departments: string[]
  locations: string[]
}

/** Filter bar for contacts — department dropdown, location dropdown, More Filters button. */
export function ContactsFilterBar({ filters, onFiltersChange, departments, locations }: ContactsFilterBarProps) {
  const update = (key: keyof ContactFilters, val: string) => {
    onFiltersChange({ ...filters, [key]: val })
  }

  return (
    <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 p-4 bg-surface-container-lowest rounded-xl
      shadow-sm border border-outline-variant/20">
      <select
        value={filters.department}
        onChange={(e) => update('department', e.target.value)}
        className="px-3 py-2.5 bg-surface-container-low border border-outline-variant/50 rounded-lg text-small text-on-surface
          cursor-pointer outline-none focus:border-primary transition-colors appearance-none"
      >
        <option value="">All Departments</option>
        {departments.map((d) => (
          <option key={d} value={d}>{d}</option>
        ))}
      </select>

      <select
        value={filters.location}
        onChange={(e) => update('location', e.target.value)}
        className="px-3 py-2.5 bg-surface-container-low border border-outline-variant/50 rounded-lg text-small text-on-surface
          cursor-pointer outline-none focus:border-primary transition-colors appearance-none"
      >
        <option value="">All Locations</option>
        {locations.map((l) => (
          <option key={l} value={l}>{l}</option>
        ))}
      </select>

      <div className="flex-1" />

      <button className="flex items-center gap-2 px-4 py-2.5 bg-surface-container-lowest border border-outline-variant
        rounded-lg text-small text-on-surface-variant font-medium hover:bg-surface-container-low
        transition-colors cursor-pointer">
        <Filter size={14} />
        More Filters
      </button>
    </div>
  )
}
