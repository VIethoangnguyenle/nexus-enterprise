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
  emptyMessage?: string
  className?: string
}

/** Styled data table with optional clickable rows and empty state. */
export function DataTable<T>({
  columns,
  data,
  keyExtractor,
  onRowClick,
  emptyMessage = 'No data',
  className = '',
}: DataTableProps<T>) {
  if (data.length === 0) {
    return (
      <div className="text-center py-8 text-text-muted text-sm">{emptyMessage}</div>
    )
  }

  return (
    <table className={`w-full border-collapse ${className}`}>
      <thead>
        <tr>
          {columns.map(col => (
            <th
              key={col.id}
              className={`text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted
                uppercase tracking-wider border-b border-border ${col.className ?? ''}`}
            >
              {col.header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map(row => (
          <tr
            key={keyExtractor(row)}
            onClick={() => onRowClick?.(row)}
            className={`border-b border-border/50 transition-colors duration-150
              ${onRowClick ? 'cursor-pointer hover:bg-bg-hover' : ''}`}
          >
            {columns.map(col => (
              <td key={col.id} className={`px-4 py-3 text-sm ${col.className ?? ''}`}>
                {col.cell(row)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
