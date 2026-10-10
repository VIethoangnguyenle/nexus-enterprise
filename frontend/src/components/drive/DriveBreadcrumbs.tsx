import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'

export interface Crumb {
  /** Folder id; absent for the root. */
  id?: string
  label: string
}

/**
 * Where the open folder sits (design/mockups/core-screens.html §2): every
 * ancestor is a real link into the URL, the folder itself is plain text. Links
 * keep the workspace in the URL and nothing else.
 */
export function DriveBreadcrumbs({ crumbs }: { crumbs: Crumb[] }) {
  return (
    <nav aria-label="Đường dẫn" className="flex items-center gap-1 min-w-0 text-small text-ink-muted">
      {crumbs.map((c, i) => {
        const last = i === crumbs.length - 1
        return (
          <span key={c.id ?? 'root'} className="flex items-center gap-1 min-w-0">
            {i > 0 && <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />}
            {last ? (
              <span aria-current="page" className="truncate">{c.label}</span>
            ) : (
              <Link
                to="/drive"
                search={(prev: DriveSearch) => folderSearch(prev, c.id)}
                className="truncate rounded-sm text-inherit no-underline hover:text-ink focus-ring
                  transition-colors duration-quick"
              >
                {c.label}
              </Link>
            )}
          </span>
        )
      })}
    </nav>
  )
}
