import { useRef, useState, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { Link, useLocation } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion, type PanInfo } from 'motion/react'
import { Bell, Check, ChevronDown, Mail, Plus, RefreshCw } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { approvalPendingOptions } from '../../hooks/useApproval'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useOpenInvitationCount } from '../../hooks/useInvitations'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useUnreadCounts } from '../../hooks/useMessaging'
import { useUnreadNotificationCount } from '../../hooks/useNotifications'
import { usePeople } from '../../hooks/usePeople'
import { useWorkspaceSwitcher } from '../../hooks/useSwitchWorkspace'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { unreadByLevel } from '../../lib/conversations'
import { formatCount } from '../../lib/format'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'
import {
  countUnit, footDestinations, mainDestinations, moduleOfPath, type NavDestination,
} from '../../lib/nav-destinations'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { Button, Pressable, Spinner, Text, spaceLetter } from '../primitives'

/** Past this share of its width, or this fast (px/s), a drag to the left closes the drawer. */
const CLOSE_DISTANCE = 0.25
const CLOSE_VELOCITY = 500

const ROW = `flex items-center gap-3.5 w-full h-13 px-3.5 rounded-full text-left text-[0.9375rem] font-medium leading-tight no-underline
  focus-ring transition-colors duration-quick hover:bg-hover active:bg-hover`

interface MainMenuDrawerProps {
  open: boolean
  onClose: () => void
  /** Thông báo opens its own full screen; the drawer closes and hands over. */
  onOpenNotifications: () => void
}

/**
 * The ☰ menu of the phone (DESIGN.md §5, Điều hướng di động): the way to every
 * part of Nexus other than Chat's own tabs, and to the other workspaces. A
 * modal layer sliding in from the left; focus rules are `useModalFocus`'s
 * (trap, first focus on the workspace block, return to ☰, Esc), a press on the
 * scrim or a drag to the left also closes it, and under reduced motion it fades.
 */
export function MainMenuDrawer({ open, onClose, onOpenNotifications }: MainMenuDrawerProps) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const entryRef = useRef<HTMLButtonElement>(null)
  useModalFocus({ active: open, surfaceRef, onClose, initialFocusRef: entryRef })

  const onDragEnd = (_: unknown, info: PanInfo) => {
    const width = surfaceRef.current?.offsetWidth || 1
    if (info.offset.x < -width * CLOSE_DISTANCE || info.velocity.x < -CLOSE_VELOCITY) onClose()
  }

  const slide = m.reduced
    ? m.scrim
    : {
        initial: { x: '-100%' },
        animate: { x: 0, transition: { duration: DURATION.layout, ease: EASE.outExpo } },
        exit: { x: '-100%', transition: { duration: DURATION.exit, ease: EASE.in } },
      }

  return createPortal(
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-modal lg:hidden">
          <motion.div
            {...m.scrim}
            data-testid="drawer-scrim"
            className="absolute inset-0 z-backdrop bg-scrim"
            onClick={onClose}
            aria-hidden="true"
          />
          <motion.div
            ref={surfaceRef}
            {...slide}
            role="dialog"
            aria-modal="true"
            aria-label="Menu chính"
            drag={m.reduced ? false : 'x'}
            dragConstraints={{ left: 0, right: 0 }}
            dragElastic={{ left: 0.4, right: 0 }}
            onDragEnd={onDragEnd}
            style={{ touchAction: 'pan-y' }}
            className="absolute inset-y-0 left-0 z-modal grid grid-rows-[auto_minmax(0,1fr)_auto] w-[min(316px,84%)]
              rounded-r-3xl bg-overlay shadow-overlay px-2.5 pt-[calc(2.5rem+env(safe-area-inset-top,0px))] pb-[calc(1rem+env(safe-area-inset-bottom,0px))]"
          >
            <DrawerBody onClose={onClose} onOpenNotifications={onOpenNotifications} entryRef={entryRef} />
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  )
}

function DrawerBody({ onClose, onOpenNotifications, entryRef }: {
  onClose: () => void
  onOpenNotifications: () => void
  entryRef: RefObject<HTMLButtonElement | null>
}) {
  const user = useAuthStore((s) => s.user)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const { pathname } = useLocation()
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal, isLoading } = useWorkspaceType()
  const people = usePeople(workspaceId)
  const me = user?.id ? people.byUserId.get(user.id) : undefined
  // A muted conversation adds only the times someone named the reader (the sidebar's rule).
  const chatUnread = unreadByLevel(useUnreadCounts().data?.channels)
  const pending = useQuery({ ...approvalPendingOptions(), enabled: !isPersonal && !isLoading }).data?.total ?? 0
  const notifications = useUnreadNotificationCount()
  const [wsOpen, setWsOpen] = useState(false)
  const active = moduleOfPath(pathname)

  const counts: Partial<Record<string, number>> = { messaging: chatUnread, approval: pending }

  const destination = (item: NavDestination) => {
    const label = item.phoneLabel ?? item.label
    const count = counts[item.id] ?? 0
    const current = active === item.id
    return (
      <Link
        key={item.id}
        to={item.to}
        onClick={() => { setActiveModule(item.id); onClose() }}
        aria-current={current ? 'page' : undefined}
        aria-label={count > 0 ? `${label}, ${count} ${countUnit(item.id)}` : undefined}
        className={`${ROW} ${current ? 'bg-accent-wash font-semibold' : 'text-ink'}`}
      >
        <Row icon={item.icon} label={label} current={current}>
          {count > 0 && (item.id === 'approval'
            ? <span className="text-xs font-medium text-ink-muted tnum">{count} chờ bạn</span>
            : <CountPill n={count} />)}
        </Row>
      </Link>
    )
  }

  return (
    <>
      <div className="flex items-center gap-2 px-3 pb-2.5 font-display text-xl font-bold leading-none text-ink">
        <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-lg bg-accent text-on-accent text-[0.9375rem]">N</span>
        Nexus
      </div>

      <div className="min-h-0 overflow-y-auto">
        <WorkspaceBlock open={wsOpen} onToggle={() => setWsOpen((o) => !o)} onDone={onClose} entryRef={entryRef} />
        <nav aria-label="Các phần của Nexus" className="grid gap-0.5">
          {mainDestinations(isPersonal).map(destination)}
          <div role="separator" className="h-px my-1.5 mx-3.5 bg-line" />
          <Pressable
            onClick={() => { onClose(); onOpenNotifications() }}
            aria-label={notifications > 0 ? `Thông báo, ${notifications} ${countUnit('notifications')}` : undefined}
            className={`${ROW} text-ink`}
          >
            <Row icon={Bell} label="Thông báo" current={false}>
              {notifications > 0 && <CountPill n={notifications} />}
            </Row>
          </Pressable>
          {footDestinations(isPersonal).map(destination)}
        </nav>
      </div>

      <div className="px-3.5 pt-3 text-sm text-ink-muted truncate">
        <span className="font-semibold text-ink">{me?.name || UNKNOWN_PERSON}</span>
        {me?.role ? ` · ${me.role}` : ''}
      </div>
    </>
  )
}

function Row({ icon: Icon, label, current, children }: { icon: LucideIcon; label: string; current: boolean; children?: ReactNode }) {
  return (
    <>
      <Icon size={20} strokeWidth={1.75} aria-hidden="true" className={current ? 'text-accent' : 'text-ink-muted'} />
      <span className="flex-1 min-w-0 truncate">{label}</span>
      {children}
    </>
  )
}

function CountPill({ n }: { n: number }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex items-center justify-center h-5 min-w-5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum"
    >
      {formatCount(n)}
    </span>
  )
}

/** Name of the open workspace; pressing it unfolds the workspaces the person can enter, in place. */
function WorkspaceBlock({ open, onToggle, onDone, entryRef }: {
  open: boolean
  onToggle: () => void
  onDone: () => void
  entryRef: RefObject<HTMLButtonElement | null>
}) {
  const { workspaceName } = useActiveWorkspace()
  const { currentId, workspaces: mine } = useWorkspaceSwitcher({ landOnChat: true })
  const { isPersonal } = useWorkspaceType()
  const offerCount = useOpenInvitationCount()
  const name = workspaceDisplayName(workspaceName)
  const current = mine.find((w) => w.id === currentId)
  const kind = isPersonal
    ? 'Workspace cá nhân · chỉ bạn'
    : current?.member_count ? `Workspace tổ chức · ${current.member_count} thành viên` : 'Workspace tổ chức'

  return (
    <div className="grid gap-1.5 mb-1.5">
      <Pressable
        ref={entryRef}
        onClick={onToggle}
        aria-expanded={open}
        aria-label={`Đổi workspace, đang ở ${name}${offerCount > 0 ? ', có lời mời' : ''}`}
        className="grid grid-cols-[2.5rem_minmax(0,1fr)_1.25rem] items-center gap-3 w-full min-h-16 px-3 py-2 rounded-2xl bg-sunk"
      >
        <span
          aria-hidden="true"
          className="relative grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold"
        >
          {spaceLetter(name)}
          {offerCount > 0 && (
            <span data-testid="invitation-dot" className="absolute -top-0.5 -right-0.5 w-3 h-3 rounded-full bg-accent ring-2 ring-sunk" />
          )}
        </span>
        <span className="grid min-w-0 gap-0.5">
          <span className="font-display text-[0.9375rem] font-bold text-ink truncate">{name}</span>
          <span className="text-xs text-ink-muted truncate">{kind}</span>
        </span>
        <ChevronDown
          size={20}
          strokeWidth={1.75}
          aria-hidden="true"
          className={`text-ink-muted transition-transform duration-quick motion-reduce:transition-none ${open ? 'rotate-180' : ''}`}
        />
      </Pressable>
      {open && <WorkspaceList onDone={onDone} offerCount={offerCount} />}
    </div>
  )
}

function WorkspaceList({ onDone, offerCount }: { onDone: () => void; offerCount: number }) {
  const { currentId, workspaces, isLoading, isError, refetch, choose, switchingId } = useWorkspaceSwitcher({ landOnChat: true })
  const busy = switchingId !== undefined
  // Where "Tạo workspace" sends Back to: the screen the person is on now.
  const here = useLocation({ select: (l) => l.href })

  if (isError) {
    return (
      <div className="grid justify-items-center gap-3 px-4 py-6 text-center">
        <Text variant="body" className="block max-w-64">
          Không tải được danh sách workspace. Kiểm tra kết nối rồi thử lại.
        </Text>
        <Button variant="soft" size="sm" onClick={() => void refetch()}>
          <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
          Thử lại
        </Button>
      </div>
    )
  }
  if (isLoading) {
    return (
      <ul className="grid gap-1 m-0 p-0" aria-busy="true">
        {[0, 1].map((i) => (
          <li key={i} data-skeleton className="list-none flex items-center gap-3 min-h-12 px-3.5">
            <div className="skeleton w-7 h-7 rounded-lg" />
            <div className="skeleton h-3.5 w-3/5 rounded-md" />
          </li>
        ))}
      </ul>
    )
  }
  return (
    <div className="grid gap-0.5">
      <ul role="listbox" aria-label="Workspace của bạn" className="grid gap-0.5 m-0 p-0">
        {workspaces.map((w) => {
          const name = workspaceDisplayName(w.name)
          const isCurrent = w.id === currentId
          return (
            <li key={w.id} role="presentation" className="list-none">
              <Pressable
                role="option"
                aria-selected={isCurrent}
                disabled={busy}
                onClick={() => choose(w.id, onDone)}
                className="grid grid-cols-[1.75rem_minmax(0,1fr)_1.25rem] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
                  text-[0.9375rem] font-medium text-ink hover:bg-hover transition-colors duration-quick"
              >
                <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-lg bg-accent text-on-accent font-display text-sm font-bold">
                  {spaceLetter(name)}
                </span>
                <span className="truncate">{name}</span>
                {switchingId === w.id
                  ? <Spinner size="sm" />
                  : isCurrent && <Check size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />}
              </Pressable>
            </li>
          )
        })}
      </ul>
      {offerCount > 0 && (
        <Link
          to="/workspace-select"
          onClick={onDone}
          className="grid grid-cols-[1.75rem_minmax(0,1fr)_auto] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
            text-[0.9375rem] font-medium text-ink no-underline hover:bg-hover transition-colors duration-quick focus-ring"
        >
          <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-lg bg-sunk text-ink-muted">
            <Mail size={16} strokeWidth={1.75} />
          </span>
          <span>Lời mời</span>
          <CountPill n={offerCount} />
          <span className="sr-only">, {offerCount} lời mời đang chờ</span>
        </Link>
      )}
      <Link
        to="/onboarding"
        search={{ from: here }}
        onClick={onDone}
        className="grid grid-cols-[1.75rem_minmax(0,1fr)] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
          text-[0.9375rem] font-medium text-accent no-underline hover:bg-hover transition-colors duration-quick focus-ring"
      >
        <span aria-hidden="true" className="grid place-items-center w-7 h-7"><Plus size={20} strokeWidth={1.75} /></span>
        <span>Tạo workspace</span>
      </Link>
    </div>
  )
}
