import type { BrowseView } from '../../hooks/useContactsBrowse'
import type { PathSegment } from './contacts-model'

export interface PhoneBar {
  title: string
  /** The path above this level, root first; the screen draws the last segment whole. */
  above: PathSegment[]
  /** What stands under the title when there is no path above it. */
  text?: string
}

/**
 * What the bar of a phone list screen says (DESIGN.md "Danh bạ", tier 2): the name of this level and, under
 * it, where it is: the departments above it, or how many it holds.
 */
export function phoneBarOf(view: BrowseView, workspaceName: string): PhoneBar {
  // The level the address names: what is typed over it does not rename the screen.
  switch (view.level.kind) {
    case 'dept': {
      if (view.atRoot) return { title: 'Phòng ban', above: [], text: workspaceName }
      const above = view.trail.slice(0, -1)
      return above.length > 0 ? { title: view.level.title, above } : { title: view.level.title, above: [], text: workspaceName }
    }
    case 'search':
      return { title: 'Tìm trong danh bạ', above: [], text: view.limited ? 'Trong phạm vi bạn được xem' : `Cả ${workspaceName}` }
    case 'role':
      return { title: view.level.title, above: [], text: `Vai trò · ${view.level.sub}` }
    default:
      return { title: view.level.title, above: [], text: view.level.sub }
  }
}
