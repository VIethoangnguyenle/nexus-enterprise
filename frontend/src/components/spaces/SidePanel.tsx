import { useRef, useState, useSyncExternalStore, type ReactNode, type Ref } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { ArrowLeft, X } from 'lucide-react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useEdgeBack } from '../../hooks/useEdgeBack'
import { isStandalone } from '../../lib/push'
import { useMotionPresets } from '../../lib/motion'
import { Heading, IconButton } from '../primitives'

interface SidePanelProps {
  /** Landmark name, e.g. "Chủ đề", "Thành viên". */
  label: string
  title: string
  sub?: ReactNode
  /** Extra header controls, before the close button. */
  actions?: ReactNode
  /** Object of "Đóng …" for the close button's name. */
  closeLabel: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  /**
   * Below which width the panel is a full-screen sheet: `md` (768) by default,
   * `lg` (1024) for the thread panels (DESIGN.md "Panel thread").
   */
  sheetBelow?: 'md' | 'lg'
  /**
   * Back to where the panel was opened from (the thread list). Beside the title
   * on a wide screen; on a sheet the back arrow replaces the close button.
   */
  onBack?: () => void
  /** Sheets lead with a back arrow instead of ending with a close button. */
  backOnSheet?: boolean
  /** The scrolling body, for panels that place the scroll themselves. */
  bodyRef?: Ref<HTMLDivElement>
  /**
   * As a sheet in the installed app, a swipe from the left edge leaves it like ← does (DESIGN.md §6 "Vuốt": the
   * thread follows the finger, the conversation shows under it). In a browser tab the edge is Safari's and ← stays.
   */
  edgeBack?: boolean
}

/** The installed app, where iOS has no back gesture of its own. A browser without `matchMedia` is not one. */
function installedApp(): boolean {
  try {
    return isStandalone()
  } catch {
    return false
  }
}

const QUERY = { md: '(max-width: 767.98px)', lg: '(max-width: 1023.98px)' } as const
const subscribeTo = (query: string) => (cb: () => void) => {
  const mq = window.matchMedia?.(query)
  mq?.addEventListener('change', cb)
  return () => mq?.removeEventListener('change', cb)
}
const WATCH = {
  md: { subscribe: subscribeTo(QUERY.md), snapshot: () => !!window.matchMedia?.(QUERY.md).matches },
  lg: { subscribe: subscribeTo(QUERY.lg), snapshot: () => !!window.matchMedia?.(QUERY.lg).matches },
}

// Whole class names, so Tailwind sees them.
const SURFACE = {
  md: `max-md:fixed max-md:inset-0 max-md:z-modal max-md:pt-safe
    md:absolute md:right-3 md:inset-y-3 md:w-90 md:rounded-surface md:shadow-overlay md:z-sticky
    xl:static xl:my-3 xl:mr-3 xl:shadow-none xl:shrink-0`,
  // h-app follows the visual viewport, so the reply box rides above the on-screen keyboard.
  lg: `max-lg:fixed max-lg:inset-x-0 max-lg:top-0 max-lg:h-app max-lg:z-modal max-lg:pt-safe
    lg:absolute lg:right-3 lg:inset-y-3 lg:w-90 lg:rounded-surface lg:shadow-overlay lg:z-sticky
    xl:static xl:my-3 xl:mr-3 xl:shadow-none xl:shrink-0`,
}

/**
 * Right detail panel (DESIGN.md §5–§7): raised, radius 10, 12px off the
 * edges, 360px. Enters with translateX(16px) scale(.985) over 280ms and leaves
 * in 210ms; under 1280px it floats over the content, under 768px it is a
 * full-screen sheet. Mount inside `AnimatePresence` for the exit.
 *
 * As a sheet it is a modal layer, so it behaves like `Dialog`: rendered in
 * `document.body` at the modal level (above the phone tab bar and clear of the
 * content area's stacking context), focus moves in and is trapped, Esc closes,
 * and focus returns to what opened it. Wider than that it stays a plain
 * complementary region beside the content.
 */
export function SidePanel({
  label, title, sub, actions, closeLabel, onClose, children, footer, sheetBelow = 'md', onBack, backOnSheet, bodyRef, edgeBack,
}: SidePanelProps) {
  const m = useMotionPresets()
  const sheet = useSyncExternalStore(WATCH[sheetBelow].subscribe, WATCH[sheetBelow].snapshot, () => false)
  const surfaceRef = useRef<HTMLElement>(null)
  useModalFocus({ active: sheet, surfaceRef, onClose })
  const [standalone] = useState(installedApp)
  const edge = !!edgeBack && sheet && standalone
  const back = useEdgeBack({ ref: surfaceRef, enabled: edge, reduced: m.reduced, onBack: onBack ?? onClose })
  const moving = edge && back.phase !== 'idle'

  const content = (
    <>
      <div className={`flex items-center gap-2 pt-3.5 pr-3 pb-2.5 ${sheet && backOnSheet ? 'pl-2' : onBack ? 'pl-2' : 'pl-4'}`}>
        {sheet && backOnSheet ? (
          <IconButton aria-label={`Đóng ${closeLabel}`} title="Đóng" onClick={onBack ?? onClose}>
            <ArrowLeft size={18} strokeWidth={1.75} />
          </IconButton>
        ) : (
          onBack && (
            <IconButton aria-label="Quay lại danh sách Thread" title="Quay lại" onClick={onBack}>
              <ArrowLeft size={18} strokeWidth={1.75} />
            </IconButton>
          )
        )}
        <div className="grid min-w-0">
          <Heading as="h2" look="section" className="truncate">{title}</Heading>
          {sub && <span className="text-xs text-ink-muted truncate">{sub}</span>}
        </div>
        <div className="ml-auto flex items-center gap-1 shrink-0">
          {actions}
          {!(sheet && backOnSheet) && (
            <IconButton aria-label={`Đóng ${closeLabel}`} title="Đóng" onClick={onClose}>
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          )}
        </div>
      </div>
      <div ref={bodyRef} className="flex-1 min-h-0 overflow-y-auto px-1 pb-2">{children}</div>
      {footer}
    </>
  )

  const panel = (
    <motion.aside
      ref={surfaceRef}
      role={sheet ? 'dialog' : 'complementary'}
      aria-modal={sheet || undefined}
      aria-label={label}
      {...m.panel}
      // While the edge swipe moves the panel the aside itself is see-through, so the conversation shows under it.
      className={`flex flex-col min-h-0 min-w-0 ${moving ? 'bg-transparent' : 'bg-raised'} ${SURFACE[sheetBelow]}`}
    >
      {edge ? (
        <motion.div
          data-testid="panel-edge-back"
          style={{ x: back.x }}
          className={`flex-1 flex flex-col min-h-0 bg-raised ${moving ? 'shadow-overlay will-change-transform' : ''}`}
        >
          {content}
        </motion.div>
      ) : content}
    </motion.aside>
  )
  return sheet ? createPortal(panel, document.body) : panel
}
