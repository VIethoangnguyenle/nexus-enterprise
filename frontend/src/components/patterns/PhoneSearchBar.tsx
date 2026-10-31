import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useOpenInvitationCount } from '../../hooks/useInvitations'
import { useChatOverlay } from '../../hooks/useChatOverlay'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { usePeople } from '../../hooks/usePeople'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { isPhoneDetailPath, moduleOfPath, phoneNameOf, searchHintOf, type ModuleId } from '../../lib/nav-destinations'
import { personStyle } from '../../lib/person-hue'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Pressable, spaceLetter } from '../primitives'
import { CreateRequestDialog } from '../approval/CreateRequestDialog'
import { UnifiedSearch } from '../search/UnifiedSearch'
import { scopeOfModule } from '../search/search-scopes'
import { WorkspaceSwitcherSheet } from './WorkspaceSwitcherSheet'

const SEARCH = '[data-module-search]'

/** The screen's own search control inside `root`, if it draws one. */
const findSearch = (root: ParentNode) => root.querySelector<HTMLElement>(SEARCH)

type Layer = 'workspaces' | 'search' | 'create-request' | null

/**
 * The modules whose pill opens the one search of every module (DESIGN.md §6 "Tìm cho mọi tác vụ"). Chat and
 * Danh bạ keep their own search screens, which carry their own filters; Cài đặt and Quản trị search their page.
 */
const UNIFIED: readonly ModuleId[] = ['drive', 'documents', 'approval', 'assets', 'calendar']

/**
 * The top of every list screen on a phone (DESIGN.md §5, Điều hướng di động):
 * a pill with the workspace's letter tile on the left, the module's search hint
 * in the middle and the person's avatar on the right. The tile opens the sheet
 * that switches workspace, the avatar the account sheet. The way to every
 * module is the bottom bar (`ModuleBar`), not this pill.
 *
 * In Tài liệu, Phê duyệt, Tài sản and Lịch the hint opens the one search of every module (`UnifiedSearch`),
 * scoped to that module. In Chat it opens the Tìm trong chat screen, which owns a history entry of `/channels`.
 * Elsewhere it goes to the search of the screen below (`[data-module-search]`): a box takes focus, a button
 * opens its panel; a screen with none gets the module's name there instead of a button that does nothing.
 * `scope` is the element holding the screen; it is watched so this follows the screen.
 */
export function PhoneSearchBar({ scope }: { scope: HTMLElement | null }) {
  // A conversation or a document carries its own header on a phone; this bar steps aside.
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  // The settings pages are full screens with their own bar and a way back (DESIGN.md §6 "Cài đặt").
  const shown = useBottomBarLayout() && !isPhoneDetailPath(pathname) && !pathname.startsWith('/settings')
  const user = useAuthStore((s) => s.user)
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const people = usePeople(workspaceId)
  const me = user?.id ? people.byUserId.get(user.id) : undefined
  // The one thing another workspace can have for this person that is known from here: an open invitation.
  const offers = useOpenInvitationCount()
  const { openSearch } = useChatOverlay()
  const navigate = useNavigate()
  const [layer, setLayer] = useState<Layer>(null)
  const [hasSearch, setHasSearch] = useState(false)
  const module = moduleOfPath(pathname)
  const unified = !!module && UNIFIED.includes(module)

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

  if (!shown) return null
  const myName = me?.name || UNKNOWN_PERSON
  const where = workspaceDisplayName(workspaceName)
  return (
    <header className="shrink-0 px-3 pb-1 pt-[calc(env(safe-area-inset-top,0px)+0.75rem)] bg-base lg:hidden">
      <div className="flex items-center gap-1 h-13 rounded-full bg-sunk">
        <Pressable
          onClick={() => setLayer('workspaces')}
          aria-haspopup="dialog"
          aria-label={`Đổi workspace, đang ở ${where}${offers > 0 ? ', có lời mời' : ''}`}
          className="relative grid place-items-center w-11 h-11 ml-1 rounded-full hover:bg-hover"
        >
          <span
            aria-hidden="true"
            // An organization is the accent; a workspace of one is the person's own hue.
            style={isPersonal && user?.id ? personStyle(user.id) : undefined}
            className={`grid place-items-center w-8 h-8 rounded-lg font-display text-16 font-bold
              ${isPersonal && user?.id ? 'bg-(--p) text-on-person' : 'bg-accent text-on-accent'}`}
          >
            {spaceLetter(where)}
          </span>
          {offers > 0 && (
            <span
              aria-hidden="true"
              data-testid="invitation-dot"
              className="absolute top-1.5 right-1.5 w-2.25 h-2.25 rounded-full bg-accent ring-2 ring-sunk"
            />
          )}
        </Pressable>
        {module === 'messaging' || unified || hasSearch ? (
          <Pressable
            onClick={module === 'messaging' ? openSearch : unified ? () => setLayer('search') : goToSearch}
            aria-haspopup={unified ? 'dialog' : undefined}
            className="flex-1 min-w-0 h-11 px-1 truncate text-16 text-ink-muted"
          >
            {searchHintOf(module)}
          </Pressable>
        ) : (
          <span className="flex-1 min-w-0 px-1 truncate text-16 text-ink-muted">{phoneNameOf(module)}</span>
        )}
        <Pressable
          onClick={() => void navigate({ to: '/settings', search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })}
          aria-label="Tôi"
          className="grid place-items-center w-11 h-11 mr-1 rounded-full hover:bg-hover"
        >
          <Avatar name={myName} hueKey={user?.id} src={me?.avatarUrl} size={32} />
        </Pressable>
      </div>

      {layer === 'workspaces' && <WorkspaceSwitcherSheet onClose={close} />}
      {layer === 'search' && (
        <UnifiedSearch
          home={scopeOfModule(module)}
          onClose={close}
          onSwitchWorkspace={() => setLayer('workspaces')}
          onCreateRequest={() => setLayer('create-request')}
        />
      )}
      <CreateRequestDialog
        open={layer === 'create-request'}
        onClose={close}
        onCreated={(request) => {
          close()
          void navigate({ to: '/approval', search: { tab: 'mine', request: request.id } })
        }}
      />
    </header>
  )
}
