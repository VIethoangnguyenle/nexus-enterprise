import { Button } from '../primitives'
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
      {/* eslint-disable-next-line no-restricted-syntax -- Select primitive bọc thẻ trong
          <div class="flex flex-col gap-1"> để chứa thông báo lỗi; ở đây select là con trực
          tiếp của hàng flex nên thẻ bọc sẽ chiếm suất flex thay cho chính nó. Xem ghi chú
          về thẻ bọc của Input/Select ở mốc nghiệm thu chặng 2. */}
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

      {/* eslint-disable-next-line no-restricted-syntax -- Cùng lý do thẻ bọc như select
          phía trên. */}
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

      <Button variant="secondary">
        <Filter size={14} />
        More Filters
      </Button>
    </div>
  )
}
