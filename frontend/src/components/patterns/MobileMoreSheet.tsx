import { useRef, useState, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion, useDragControls, type PanInfo } from 'motion/react'
import {
  ArrowLeft, Check, ChevronRight, LogOut, Package, RefreshCw, Settings, ShieldCheck, Users, X,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { logoutSession } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useModalFocus } from '../../hooks/useModalFocus'
import { usePeople } from '../../hooks/usePeople'
import { useWorkspaceSwitcher } from '../../hooks/useSwitchWorkspace'
import { workspaceRowDetail } from '../../lib/auth-flow'
import { useMotionPresets } from '../../lib/motion'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { Avatar, Button, Heading, IconButton, Pressable, Spinner, Text, spaceLetter } from '../primitives'

type MoreItem = { id: 'assets' | 'contacts' | 'admin' | 'settings'; icon: LucideIcon; label: string; to: string }

/** DESIGN.md §5, Điều hướng di động: what the bar has no room for, in this order. */
export const MORE_ITEMS: MoreItem[] = [
  { id: 'assets', icon: Package, label: 'Tài sản', to: '/assets' },
  { id: 'contacts', icon: Users, label: 'Danh bạ', to: '/contacts' },
  { id: 'admin', icon: ShieldCheck, label: 'Quản trị', to: '/admin' },
  { id: 'settings', icon: Settings, label: 'Cài đặt', to: '/settings' },
]

/** Past this share of its height, or this fast (px/s), a downward drag closes the sheet. */
const CLOSE_DISTANCE = 0.25
const CLOSE_VELOCITY = 500

interface MobileMoreSheetProps {
  open: boolean
  onClose: () => void
  /** The path being shown, to mark the item the person is already on. */
  pathname: string
}

/**
 * The "Thêm" sheet of the phone tab bar (DESIGN.md §5): a modal bottom sheet
 * holding the current workspace and person, the destinations the bar has no
 * room for, and Đăng xuất. The workspace row swaps the content for the list of
 * workspaces the person can enter. Focus rules come from `useModalFocus`
 * (trap, initial focus, return focus, Esc), the same ones `Dialog` and the
 * phone `SidePanel` follow; a downward drag on the handle row also closes it.
 */
export function MobileMoreSheet({ open, onClose, pathname }: MobileMoreSheetProps) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const dragControls = useDragControls()
  // Focus lands on the workspace row (the first thing in the content), not on the close button.
  const entryRef = useRef<HTMLButtonElement>(null)
  useModalFocus({ active: open, surfaceRef, onClose, initialFocusRef: entryRef })

  const onDragEnd = (_: unknown, info: PanInfo) => {
    const height = surfaceRef.current?.offsetHeight || 1
    if (info.offset.y > height * CLOSE_DISTANCE || info.velocity.y > CLOSE_VELOCITY) onClose()
  }

  return createPortal(
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-modal lg:hidden">
          <motion.div {...m.scrim} className="absolute inset-0 z-backdrop bg-scrim" onClick={onClose} aria-hidden="true" />
          <motion.div
            ref={surfaceRef}
            {...m.sheet}
            role="dialog"
            aria-modal="true"
            aria-label="Thêm"
            drag="y"
            dragControls={dragControls}
            dragListener={false}
            dragConstraints={{ top: 0, bottom: 0 }}
            dragElastic={{ top: 0, bottom: 0.5 }}
            onDragEnd={onDragEnd}
            className="absolute inset-x-0 bottom-0 z-modal flex flex-col max-h-sheet overflow-hidden rounded-t-overlay
              bg-overlay shadow-overlay px-4 pb-sheet"
          >
            <SheetBody onClose={onClose} pathname={pathname} entryRef={entryRef} onHandleDown={(e) => dragControls.start(e)} />
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  )
}

function SheetBody({ onClose, pathname, entryRef, onHandleDown }: {
  onClose: () => void
  pathname: string
  entryRef: RefObject<HTMLButtonElement | null>
  onHandleDown: (e: React.PointerEvent) => void
}) {
  const [view, setView] = useState<'menu' | 'workspaces'>('menu')
  return (
    <>
      {/* The handle row is the drag surface; the content below scrolls. */}
      <div
        onPointerDown={onHandleDown}
        className="grid grid-cols-[2rem_1fr_2rem] items-center h-10 mt-1 shrink-0 touch-none"
      >
        {view === 'workspaces' && (
          <IconButton aria-label="Quay lại" onClick={() => setView('menu')} className="col-start-1 row-start-1">
            <ArrowLeft size={18} strokeWidth={1.75} />
          </IconButton>
        )}
        {view === 'workspaces' ? (
          <Heading as="h2" look="section" className="col-start-2 row-start-1 justify-self-center">Đổi workspace</Heading>
        ) : (
          <span aria-hidden="true" className="col-start-2 row-start-1 justify-self-center w-9 h-1 rounded-full bg-line" />
        )}
        <IconButton aria-label="Đóng Thêm" onClick={onClose} className="col-start-3 row-start-1">
          <X size={18} strokeWidth={1.75} />
        </IconButton>
      </div>
      <div className="min-h-0 overflow-y-auto">
        {view === 'menu'
          ? <MenuView onClose={onClose} pathname={pathname} entryRef={entryRef} onPickWorkspace={() => setView('workspaces')} />
          : <WorkspaceView onDone={onClose} />}
      </div>
    </>
  )
}

function MenuView({ onClose, pathname, entryRef, onPickWorkspace }: {
  onClose: () => void
  pathname: string
  entryRef: RefObject<HTMLButtonElement | null>
  onPickWorkspace: () => void
}) {
  const user = useAuthStore((s) => s.user)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const me = user?.id ? people.byUserId.get(user.id) : undefined
  const name = workspaceDisplayName(workspaceName)

  return (
    <div className="grid gap-1">
      <Pressable
        ref={entryRef}
        onClick={onPickWorkspace}
        aria-label={`Đổi workspace, đang ở ${name}`}
        className="grid grid-cols-[2.5rem_minmax(0,1fr)_auto] items-center gap-3 w-full min-h-16 p-3 rounded-surface
          bg-raised hover:bg-hover transition-colors duration-quick"
      >
        <span
          aria-hidden="true"
          className="grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold"
        >
          {spaceLetter(name)}
        </span>
        <span className="grid min-w-0 gap-0.5">
          <span className="font-display text-base font-bold text-ink truncate">{name}</span>
          <span className="flex items-center gap-1.5 min-w-0 text-sm text-ink-muted">
            <Avatar name={me?.name || UNKNOWN_PERSON} hueKey={user?.id} src={me?.avatarUrl} size={20} />
            <span className="truncate">{me?.name || UNKNOWN_PERSON}{me?.role ? ` · ${me.role}` : ''}</span>
          </span>
        </span>
        <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
      </Pressable>

      <nav aria-label="Thêm" className="grid gap-0.5 pt-2">
        {MORE_ITEMS.map((item) => {
          const active = pathname === item.to || pathname.startsWith(`${item.to}/`)
          return (
            <Link
              key={item.id}
              to={item.to}
              onClick={() => {
                setActiveModule(item.id)
                onClose()
              }}
              aria-current={active ? 'page' : undefined}
              className={`flex items-center gap-3 h-12 px-3 rounded-surface text-base no-underline focus-ring
                transition-colors duration-quick hover:bg-hover
                ${active ? 'bg-raised text-ink font-semibold' : 'text-ink font-medium'}`}
            >
              <item.icon size={18} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
              {item.label}
            </Link>
          )
        })}
        <div role="separator" className="h-px my-1 bg-line" />
        <Pressable
          onClick={() => {
            onClose()
            void logoutSession()
          }}
          className="flex items-center gap-3 h-12 px-3 rounded-surface text-base font-medium text-ink
            hover:bg-hover transition-colors duration-quick"
        >
          <LogOut size={18} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
          Đăng xuất
        </Pressable>
      </nav>
    </div>
  )
}

function WorkspaceView({ onDone }: { onDone: () => void }) {
  const { currentId, workspaces, isLoading, isError, refetch, choose, switchingId } = useWorkspaceSwitcher()
  const busy = switchingId !== undefined

  if (isError) {
    return (
      <div className="grid justify-items-center gap-3 px-4 py-8 text-center">
        <Text variant="body" className="block max-w-72">
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
          <li key={i} data-skeleton className="list-none flex items-center gap-3 min-h-14 px-3">
            <div className="skeleton w-10 h-10 rounded-surface" />
            <div className="grid flex-1 gap-2">
              <div className="skeleton h-3.5 w-3/5 rounded-md" />
              <div className="skeleton h-2.5 w-2/5 rounded-md" />
            </div>
          </li>
        ))}
      </ul>
    )
  }
  return (
    <ul role="listbox" aria-label="Workspace của bạn" className="grid gap-0.5 m-0 p-0">
      {workspaces.map((w) => {
        const name = workspaceDisplayName(w.name)
        const current = w.id === currentId
        return (
          <li key={w.id} role="presentation" className="list-none">
            <Pressable
              role="option"
              aria-selected={current}
              disabled={busy}
              onClick={() => choose(w.id, onDone)}
              className={`grid grid-cols-[2.5rem_minmax(0,1fr)_1.25rem] items-center gap-3 w-full min-h-14 px-3 py-2
                rounded-surface hover:bg-hover transition-colors duration-quick ${current ? 'bg-raised' : ''}`}
            >
              <span
                aria-hidden="true"
                className="grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold"
              >
                {spaceLetter(name)}
              </span>
              <span className="grid min-w-0">
                <span className="font-semibold text-ink truncate">{name}</span>
                <span className="text-xs text-ink-muted truncate">{workspaceRowDetail(w)}</span>
              </span>
              {switchingId === w.id
                ? <Spinner size="sm" />
                : current && <Check size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />}
            </Pressable>
          </li>
        )
      })}
    </ul>
  )
}
