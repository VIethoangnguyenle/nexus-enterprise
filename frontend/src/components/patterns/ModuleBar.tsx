import { useEffect, useRef, useState, type MouseEvent, type PointerEvent } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { LayoutGrid, Plus } from 'lucide-react'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { useChatChip } from '../../hooks/useChatChip'
import { useKeyboardOpen } from '../../hooks/useKeyboardOpen'
import { useLongPress } from '../../hooks/useLongPress'
import { useModuleBar, useModuleNews } from '../../hooks/useModuleBar'
import { useModuleCreateAction } from '../../hooks/useModuleCreateAction'
import { useScrollCompact } from '../../hooks/useScrollCompact'
import { formatCount } from '../../lib/format'
import { useMotionPresets } from '../../lib/motion'
import { countUnit, isModuleBarPath, moduleOfPath, type BarModule } from '../../lib/nav-destinations'
import { useUiStore } from '../../stores/ui.store'
import { Pressable } from '../primitives'
import { NewChatButton } from '../spaces/NewChatButton'
import { ChatQuickJump } from './ChatQuickJump'
import { EditBarScreen } from './EditBarScreen'
import { MoreSheet } from './MoreSheet'
import { NotificationScreen } from './NotificationScreen'
import { WorkspaceSwitcherSheet } from './WorkspaceSwitcherSheet'

type Layer = 'more' | 'edit' | 'jump' | 'notifications' | 'workspaces' | null

/** Matches no control: a long press on any slot of the bar counts. */
const PRESS_ANYWHERE = '[data-never]'

/**
 * The phone's bottom bar (DESIGN.md §5, Điều hướng di động): on every list screen below 1024px, never on a
 * screen with a way back of its own, on the settings pages, or with the keyboard open. Where the desktop's
 * sidebar is shown it is not drawn at all, and nothing below it asks for any data.
 *
 * `scope` is the element holding the screen; the bar folds to icons when that screen is scrolled down.
 */
export function ModuleBar({ scope }: { scope: HTMLElement | null }) {
  const pathname = useLocation({ select: (l) => l.pathname })
  const keyboardOpen = useKeyboardOpen()
  return useBottomBarLayout() && isModuleBarPath(pathname) && !keyboardOpen ? <Bar scope={scope} pathname={pathname} /> : null
}

function Bar({ scope, pathname }: { scope: HTMLElement | null; pathname: string }) {
  const m = useMotionPresets()
  const bar = useModuleBar()
  const news = useModuleNews(bar.ready, bar.isPersonal)
  const { compact, scrollToTop } = useScrollCompact(scope, pathname, !m.reduced)
  const { chip, select } = useChatChip()
  const setChatBarShown = useUiStore((s) => s.setChatBarShown)
  const [layer, setLayer] = useState<Layer>(null)
  const close = () => setLayer(null)
  const current = moduleOfPath(pathname)

  // What floats at the bottom edge (toasts, the update notice) lifts itself above the bar while it is here.
  useEffect(() => {
    setChatBarShown(true)
    return () => setChatBarShown(false)
  }, [setChatBarShown])

  // A long press: the Chat slot opens Đi nhanh, anywhere else on the bar opens Sửa thanh đáy. The tap
  // that ends it is swallowed so the slot under the finger does not also open.
  const pressed = useRef<string | undefined>(undefined)
  const swallow = useRef(false)
  const press = useLongPress(() => {
    swallow.current = true
    setLayer(pressed.current === 'chat' ? 'jump' : 'edit')
  }, true, PRESS_ANYWHERE)

  if (!bar.ready) return null

  const inMore = bar.rest.some((r) => news[r.id] > 0)
  const labelOf = (item: BarModule) => {
    const n = news[item.id]
    return n > 0 ? `${item.label}, ${n} ${countUnit(item.id)}` : item.label
  }

  // Pressing the slot you are in goes to the top of its list; once there, Chat's second press drops its filter.
  const onSlotPress = (item: BarModule, e: MouseEvent) => {
    if (item.id === 'chat' && current === 'messaging' && pathname.replace(/\/$/, '') === '/channels') {
      e.preventDefault()
      if (scrollToTop(!m.reduced) && chip !== 'all') select('all')
    } else if (item.module && item.module === current) {
      e.preventDefault()
      scrollToTop(!m.reduced)
    }
  }

  const slotClass = (active: boolean) =>
    `relative grid justify-items-center content-center gap-0.5 h-full rounded-full no-underline focus-ring
      ${active ? 'text-accent' : 'text-ink-muted'}`
  const pillClass = (active: boolean) =>
    `relative grid place-items-center w-12 rounded-full transition-[height,background-color] duration-[240ms] ease-out
      motion-reduce:transition-none ${compact ? 'h-9' : 'h-7.5'} ${active ? 'bg-accent-wash' : ''}`

  const slotBody = (item: BarModule, active: boolean, dot = false) => {
    const n = news[item.id]
    return (
      <>
        <span className={pillClass(active)}>
          <item.icon size={24} strokeWidth={active ? 2.1 : 1.75} aria-hidden="true" />
          {n > 0 && (
            <span
              aria-hidden="true"
              className="absolute -top-1 right-0 inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full
                bg-accent text-on-accent text-2xs font-semibold tnum ring-2 ring-overlay"
            >
              {formatCount(n)}
            </span>
          )}
          {dot && <span aria-hidden="true" data-testid="more-dot" className="absolute top-0 right-1.5 w-2.5 h-2.5 rounded-full bg-accent ring-2 ring-overlay" />}
        </span>
        <span
          aria-hidden="true"
          className={`text-[0.6875rem] leading-none transition-[opacity,height] duration-[240ms] motion-reduce:transition-none
            ${compact ? 'h-0 opacity-0 overflow-hidden' : 'h-3 opacity-100'} ${active ? 'font-semibold' : 'font-medium'}`}
        >
          {item.label}
        </span>
      </>
    )
  }

  return (
    <>
      <nav
        aria-label="Phân hệ"
        {...press}
        onPointerDownCapture={(e: PointerEvent) => {
          swallow.current = false
          pressed.current = (e.target as HTMLElement).closest<HTMLElement>('[data-bar-id]')?.dataset.barId
        }}
        onClickCapture={(e: MouseEvent) => {
          if (!swallow.current) return
          swallow.current = false
          e.preventDefault()
          e.stopPropagation()
        }}
        className={`fixed left-3.5 right-22.5 bottom-[calc(1.375rem+env(safe-area-inset-bottom,0px))] z-sticky lg:hidden
          grid grid-cols-5 items-center px-1.5 rounded-full bg-overlay/90 backdrop-blur-lg shadow-overlay
          ring-1 ring-inset ring-line select-none [-webkit-touch-callout:none]
          transition-[height] duration-[240ms] ease-out motion-reduce:transition-none ${compact ? 'h-13' : 'h-16'}`}
      >
        {bar.slots.map((item) => {
          const active = !!item.module && item.module === current
          return item.to ? (
            <Link
              key={item.id}
              to={item.to}
              data-bar-id={item.id}
              onClick={(e) => onSlotPress(item, e)}
              aria-current={active ? 'page' : undefined}
              aria-label={labelOf(item)}
              className={slotClass(active)}
            >
              {slotBody(item, active)}
            </Link>
          ) : (
            <Pressable
              key={item.id}
              data-bar-id={item.id}
              onClick={() => setLayer('notifications')}
              aria-haspopup="dialog"
              aria-label={labelOf(item)}
              className={slotClass(false)}
            >
              {slotBody(item, false)}
            </Pressable>
          )
        })}
        <Pressable
          data-bar-id="more"
          onClick={() => setLayer('more')}
          aria-haspopup="dialog"
          aria-label={inMore ? 'Thêm, có việc mới' : 'Thêm'}
          className={slotClass(false)}
        >
          {slotBody({ id: 'notifications', icon: LayoutGrid, label: 'Thêm' }, false, inMore)}
        </Pressable>
      </nav>

      {/* The + beside the bar: the create action of the module. */}
      {current === 'messaging' && <NewChatButton />}
      {current !== 'messaging' && <CreateActionFab />}

      {layer === 'more' && (
        <MoreSheet
          modules={bar.modules}
          pinned={bar.ids}
          news={news}
          onClose={close}
          onEdit={() => setLayer('edit')}
          onSwitchWorkspace={() => setLayer('workspaces')}
          onOpenNotifications={() => setLayer('notifications')}
        />
      )}
      {layer === 'jump' && <ChatQuickJump onClose={close} />}
      {layer === 'workspaces' && <WorkspaceSwitcherSheet onClose={close} />}
      {layer === 'edit' && (
        <EditBarScreen modules={bar.modules} bar={bar.ids} defaults={bar.defaults} onSave={bar.saveBar} onClose={close} />
      )}
      <NotificationScreen open={layer === 'notifications'} onClose={close} />
    </>
  )
}

/**
 * The + beside the bar for non-messaging modules: reads the create-action
 * registry and shows the button with the same position and style as NewChatButton.
 * Renders nothing when the active module has no registered create action.
 */
export function CreateActionFab() {
  const action = useModuleCreateAction()
  if (!action) return null
  return (
    <Pressable
      onClick={action.handler}
      aria-label={action.label}
      className="fixed right-3.5 bottom-[calc(1.375rem+env(safe-area-inset-bottom,0px))] z-sticky grid place-items-center w-16 h-16
        rounded-[1.25rem] bg-accent-wash text-accent shadow-overlay transition-transform duration-press active:scale-95
        motion-reduce:transition-none lg:hidden"
    >
      <Plus size={28} strokeWidth={2} aria-hidden="true" />
    </Pressable>
  )
}
