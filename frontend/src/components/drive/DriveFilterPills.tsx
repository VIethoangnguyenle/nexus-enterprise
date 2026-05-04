export type FileTypeFilter = 'all' | 'doc' | 'sheet' | 'slide' | 'pdf'

interface DriveFilterPillsProps {
  active: FileTypeFilter
  onChange: (filter: FileTypeFilter) => void
}

const filters: { id: FileTypeFilter; label: string; emoji?: string }[] = [
  { id: 'all', label: 'All Types' },
  { id: 'doc', label: 'Documents', emoji: '📄' },
  { id: 'sheet', label: 'Sheets', emoji: '📊' },
  { id: 'slide', label: 'Slides', emoji: '📑' },
  { id: 'pdf', label: 'PDF', emoji: '📕' },
]

/** Horizontal file-type filter pills — Stitch design. */
export function DriveFilterPills({ active, onChange }: DriveFilterPillsProps) {
  return (
    <div className="flex items-center gap-1.5">
      {filters.map((f) => (
        <button
          key={f.id}
          onClick={() => onChange(f.id)}
          className={`h-[30px] px-3 rounded-full text-[12px] font-medium border transition-all duration-150
            cursor-pointer flex items-center gap-1.5
            ${active === f.id
              ? 'bg-primary-container text-on-primary-container border-primary-container shadow-[0_1px_2px_rgba(0,0,0,0.04)]'
              : 'bg-transparent text-on-surface-variant/70 border-outline-variant/50 hover:bg-surface-container hover:text-on-surface hover:border-outline-variant'
            }`}
        >
          {f.emoji && <span className="text-[11px]">{f.emoji}</span>}
          {f.label}
        </button>
      ))}
    </div>
  )
}

/** Check if a MIME type matches a file type filter. */
export function matchesFileTypeFilter(mimeType: string, filter: FileTypeFilter): boolean {
  if (filter === 'all') return true

  const mime = mimeType.toLowerCase()
  switch (filter) {
    case 'doc':
      return mime.includes('document') || mime.includes('msword') || mime.includes('text/')
    case 'sheet':
      return mime.includes('spreadsheet') || mime.includes('excel') || mime.includes('csv')
    case 'slide':
      return mime.includes('presentation') || mime.includes('powerpoint')
    case 'pdf':
      return mime === 'application/pdf'
    default:
      return true
  }
}
