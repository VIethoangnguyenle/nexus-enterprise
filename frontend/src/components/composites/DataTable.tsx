import type { ReactNode } from 'react'

interface Column<T> {
  id: string
  header: string
  cell: (row: T) => ReactNode
  className?: string
}

interface DataTableProps<T> {
  columns: Column<T>[]
  data: T[]
  keyExtractor: (row: T) => string
  onRowClick?: (row: T) => void
  selectedKey?: string | null
  emptyMessage?: string
  className?: string
}

/** Dense data table — 36px rows, caption-ui headers, border-subtle dividers. */
export function DataTable<T>({
  columns,
  data,
  keyExtractor,
  onRowClick,
  selectedKey,
  emptyMessage = 'No data',
  className = '',
}: DataTableProps<T>) {
  if (data.length === 0) {
    return (
      <div className="text-center py-8 text-on-surface-variant text-small">{emptyMessage}</div>
    )
  }

  return (
    <div className="overflow-x-auto">
    <table className={`w-full border-collapse ${className}`}>
      <thead>
        <tr className="bg-surface-container">
          {columns.map(col => (
            <th
              key={col.id}
              className={`text-left px-3 py-2 text-caption-ui text-on-surface-variant
                uppercase tracking-wider border-b border-outline-variant ${col.className ?? ''}`}
            >
              {col.header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map(row => {
          const key = keyExtractor(row)
          const isSelected = selectedKey === key
          return (
            <tr
              key={key}
              onClick={() => onRowClick?.(row)}
              className={`border-b border-border-subtle transition-colors duration-instant h-9
                ${onRowClick ? 'cursor-pointer' : ''}
                ${isSelected ? 'bg-accent-bg' : 'hover:bg-surface-container-high'}`}
            >
              {columns.map(col => (
                <td key={col.id} className={`px-3 py-1 text-small text-on-surface ${col.className ?? ''}`}>
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          )
        })}
      </tbody>
    </table>
    </div>
  )
}
