import { useCallback, useEffect, useState } from 'react'
import { useRouterState } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Menu } from 'lucide-react'
import { approvalPendingOptions } from '../../hooks/useApproval'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useUnreadNotificationCount } from '../../hooks/useNotifications'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { usePeople } from '../../hooks/usePeople'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { isPhoneDetailPath, moduleOfPath, phoneNameOf, searchHintOf } from '../../lib/nav-destinations'
import { UNKNOWN_PERSON } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Pressable } from '../primitives'
import { AccountSheet } from './AccountSheet'
import { MainMenuDrawer } from './MainMenuDrawer'
import { NotificationScreen } from './NotificationScreen'

const SEARCH = '[data-module-search]'

/** The screen's own search control inside `root`, if it draws one. */
const findSearch = (root: ParentNode) => root.querySelector<HTMLElement>(SEARCH)

type Layer = 'menu' | 'account' | 'notifications' | null

/**
 * The top of every list screen on a phone (DESIGN.md §5, Điều hướng di động):
 * a pill with ☰ on the left, the module's search hint in the middle and the
 * person's avatar on the right. ☰ opens the main menu, the avatar the account
 * sheet; the menu's Thông báo opens the notifications screen.
 *
 * The hint goes to the search of the screen below (`[data-module-search]`):
 * a box takes focus, a button opens its panel. A screen with none gets the
 * module's name there instead of a button that does nothing. `scope` is the
 * element holding the screen; it is watched so this follows the screen.
 */
export function PhoneSearchBar({ scope }: { scope: HTMLElement | null }) {
  // A conversation or a document carries its own header on a phone; this bar steps aside.
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const shown = useBottomBarLayout() && !isPhoneDetailPath(pathname)
  const user = useAuthStore((s) => s.user)
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal, isLoading } = useWorkspaceType()
  const people = usePeople(workspaceId)
  const me = user?.id ? people.byUserId.get(user.id) : undefined
  const pending = useQuery({ ...approvalPendingOptions(), enabled: shown && !isPersonal && !isLoading }).data?.total ?? 0
  const unreadNotifications = useUnreadNotificationCount()
  const [layer, setLayer] = useState<Layer>(null)
  const [hasSearch, setHasSearch] = useState(false)
  const module = moduleOfPath(pathname)
  const news = pending > 0 || unreadNotifications > 0

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

  const close = useCallback(() => setLayer(null), [])
  const openNotifications = useCallback(() => setLayer('notifications'), [])

  if (!shown) return null
  const myName = me?.name || UNKNOWN_PERSON
  return (
    <header className="shrink-0 px-3 pb-1 pt-[calc(env(safe-area-inset-top,0px)+0.75rem)] bg-base lg:hidden">
      <div className="flex items-center gap-1 h-13 rounded-full bg-sunk">
        <Pressable
          onClick={() => setLayer('menu')}
          aria-haspopup="dialog"
          aria-expanded={layer === 'menu'}
          aria-label={news ? 'Mở menu chính, có việc mới' : 'Mở menu chính'}
          className="relative grid place-items-center w-11 h-11 ml-1 rounded-full text-ink hover:bg-hover"
        >
          <Menu size={20} strokeWidth={1.75} aria-hidden="true" />
          {news && (
            <span
              aria-hidden="true"
              data-testid="menu-dot"
              className="absolute top-2 right-2 w-2 h-2 rounded-full bg-accent ring-2 ring-sunk"
            />
          )}
        </Pressable>
        {hasSearch ? (
          <Pressable onClick={goToSearch} className="flex-1 min-w-0 h-11 px-1 truncate text-base text-ink-muted">
            {searchHintOf(module)}
          </Pressable>
        ) : (
          <span className="flex-1 min-w-0 px-1 truncate text-base text-ink-muted">{phoneNameOf(module)}</span>
        )}
        <Pressable
          onClick={() => setLayer('account')}
          aria-haspopup="dialog"
          aria-label="Tài khoản của bạn"
          className="grid place-items-center w-11 h-11 mr-1 rounded-full hover:bg-hover"
        >
          <Avatar name={myName} hueKey={user?.id} src={me?.avatarUrl} size={32} />
        </Pressable>
      </div>

      <MainMenuDrawer open={layer === 'menu'} onClose={close} onOpenNotifications={openNotifications} />
      {layer === 'account' && <AccountSheet onClose={close} />}
      <NotificationScreen open={layer === 'notifications'} onClose={close} />
    </header>
  )
}
