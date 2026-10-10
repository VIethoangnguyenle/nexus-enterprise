import { useEffect, useRef, type ReactNode } from 'react'
import { create } from 'zustand'
import { AnimatePresence, motion } from 'motion/react'
import { CircleCheck, CircleAlert, Info } from 'lucide-react'
import { useMotionPresets } from '../../lib/motion'

type Tone = 'success' | 'error' | 'info'

export interface ToastAction {
  label: string
  onClick: () => void
}

export interface ToastItem {
  id: number
  message: string
  tone: Tone
  action?: ToastAction
  /** Replaces the tone's icon, e.g. the avatar of the person a notification is about. */
  icon?: ReactNode
  /** Milliseconds before it leaves on its own. */
  duration: number
}

interface ToastState {
  toasts: ToastItem[]
  push: (t: Omit<ToastItem, 'id'>) => number
  dismiss: (id: number) => void
  clear: () => void
}

/** Most toasts on screen at once (DESIGN.md §6). */
const MAX_TOASTS = 3
/** Auto-dismiss after 5s (DESIGN.md §6). */
export const TOAST_DURATION = 5000

let nextId = 1

/** Client-only UI state, so it lives in Zustand rather than TanStack Query. */
export const useToastStore = create<ToastState>()((set) => ({
  toasts: [],
  push: (t) => {
    const id = nextId++
    set((s) => ({ toasts: [...s.toasts, { ...t, id }].slice(-MAX_TOASTS) }))
    return id
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
  clear: () => set({ toasts: [] }),
}))

interface ToastOptions {
  tone?: Tone
  /** e.g. `{ label: 'Hoàn tác', onClick: undo }`. Clicking also dismisses. */
  action?: ToastAction
  icon?: ReactNode
  duration?: number
}

/**
 * Show a toast. Say what happened ("Đã tạo nhóm"); for errors, say what went
 * wrong and how to fix it.
 */
export function toast(message: string, opts: ToastOptions = {}): number {
  return useToastStore.getState().push({
    message,
    tone: opts.tone ?? 'success',
    action: opts.action,
    icon: opts.icon,
    duration: opts.duration ?? TOAST_DURATION,
  })
}
toast.error = (message: string, opts: Omit<ToastOptions, 'tone'> = {}) => toast(message, { ...opts, tone: 'error' })
toast.info = (message: string, opts: Omit<ToastOptions, 'tone'> = {}) => toast(message, { ...opts, tone: 'info' })

const toneIcon = {
  success: <CircleCheck size={18} strokeWidth={1.75} className="text-success shrink-0" />,
  error: <CircleAlert size={18} strokeWidth={1.75} className="text-danger shrink-0" />,
  info: <Info size={18} strokeWidth={1.75} className="text-info shrink-0" />,
}

function ToastView({ item }: { item: ToastItem }) {
  const dismiss = useToastStore((s) => s.dismiss)
  const m = useMotionPresets()
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const start = () => {
    timer.current = setTimeout(() => dismiss(item.id), item.duration)
  }
  const stop = () => {
    if (timer.current) clearTimeout(timer.current)
  }
  useEffect(() => {
    start()
    return stop
    // The timer belongs to this toast's lifetime only, so it runs once on mount.
  }, [])

  return (
    <motion.div
      layout={m.layoutProp}
      {...m.toast}
      role={item.tone === 'error' ? 'alert' : 'status'}
      onPointerEnter={stop}
      onPointerLeave={start}
      onFocus={stop}
      onBlur={start}
      className="pointer-events-auto flex items-center gap-3 min-w-72 max-w-100 py-2.5 pr-2.5 pl-3.5
        rounded-overlay bg-overlay shadow-overlay text-sm text-ink"
    >
      {item.icon ?? toneIcon[item.tone]}
      <span className="flex-1 min-w-0">{item.message}</span>
      {item.action && (
        <button
          type="button"
          onClick={() => {
            item.action!.onClick()
            dismiss(item.id)
          }}
          className="press shrink-0 h-8 px-3 rounded-md border-none bg-transparent cursor-pointer
            text-small font-semibold text-ink hover:bg-hover focus-ring"
        >
          {item.action.label}
        </button>
      )}
    </motion.div>
  )
}

/** Mount once near the root. Bottom-centre on mobile, bottom-right on desktop. */
export function Toaster() {
  const toasts = useToastStore((s) => s.toasts)
  return (
    <div
      className="fixed z-toast bottom-4 max-lg:bottom-above-bar inset-x-4 md:inset-x-auto md:right-4 flex flex-col items-center
        md:items-end gap-2 pointer-events-none"
      aria-live="polite"
    >
      <AnimatePresence initial={false}>
        {toasts.map((t) => <ToastView key={t.id} item={t} />)}
      </AnimatePresence>
    </div>
  )
}
