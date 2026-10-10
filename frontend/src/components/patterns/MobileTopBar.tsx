import { useCallback, useEffect, useState } from 'react'
import { Search } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { workspaceDisplayName } from '../../lib/workspace'
import { IconButton } from '../primitives'

const SEARCH = '[data-module-search]'

/** The screen's own search control inside `root`, if it draws one. */
const findSearch = (root: ParentNode) => root.querySelector<HTMLElement>(SEARCH)

/**
 * Phone top bar (DESIGN.md §5, Điều hướng di động): "Nexus · <workspace>" and,
 * when the screen below has a search of its own, a search action that goes to
 * it. Screens without one get no action. The screen's title stays below.
 *
 * `scope` is the element holding the screen; it is watched so the action
 * appears and disappears with the screen's own search.
 */
export function MobileTopBar({ scope }: { scope: HTMLElement | null }) {
  const shown = useBottomBarLayout()
  const { workspaceName } = useActiveWorkspace()
  const [hasSearch, setHasSearch] = useState(false)

  useEffect(() => {
    if (!scope || !shown) return
    const sync = () => setHasSearch(!!findSearch(scope))
    sync()
    const observer = new MutationObserver(sync)
    observer.observe(scope, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [scope, shown])

  const goToSearch = useCallback(() => {
    const target = scope && findSearch(scope)
    if (!target) return
    target.scrollIntoView?.({ block: 'nearest' })
    // A search box takes focus (and the keyboard); a search button opens its panel.
    if (target instanceof HTMLInputElement) target.focus()
    else target.click()
  }, [scope])

  if (!shown) return null
  return (
    <header className="flex items-center gap-2 min-h-12 px-4 shrink-0 pt-safe bg-base lg:hidden">
      <span className="min-w-0 flex-1 truncate text-base text-ink-muted">
        <span className="font-display font-bold text-ink">Nexus</span>
        {' · '}
        {workspaceDisplayName(workspaceName)}
      </span>
      {hasSearch && (
        <IconButton size="lg" className="min-w-11 min-h-11" aria-label="Tìm kiếm" title="Tìm kiếm" onClick={goToSearch}>
          <Search size={20} strokeWidth={1.75} />
        </IconButton>
      )}
    </header>
  )
}
