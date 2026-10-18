import { forwardRef, type HTMLAttributes, type ReactNode } from 'react'
import { usePullToRefresh } from '../../hooks/usePullToRefresh'
import { Spinner } from '../primitives'

const HELD = 40

type DivProps = Omit<HTMLAttributes<HTMLDivElement>, 'onTouchStart' | 'onTouchMove' | 'onTouchEnd' | 'onTouchCancel' | 'children'>

/**
 * A list's scrolling element with pull to refresh (DESIGN.md §6 "Vuốt": "kéo xuống ở đầu mọi danh sách để tải lại"):
 * pulled down from its top it shows a spinner that follows the finger, and past the threshold it reloads and holds
 * the spinner until the reload settles. The pull is touch-only, so a desktop scrolls it like any element. Every
 * other prop is the scroller's own (its class, role, ids).
 */
export const PullToRefresh = forwardRef<HTMLDivElement, DivProps & {
  onRefresh: () => Promise<unknown>
  /** Off: a plain scroller (while the list cannot reload). */
  enabled?: boolean
  inert?: boolean
  children: ReactNode
}>(function PullToRefresh({ onRefresh, enabled = true, children, ...rest }, ref) {
  const { bind, pull, refreshing } = usePullToRefresh(onRefresh, enabled)
  return (
    <div ref={ref} {...rest} {...bind}>
      {(pull > 0 || refreshing) && (
        <div
          role="status"
          aria-label="Đang tải lại"
          style={{ height: refreshing ? HELD : pull }}
          className="grid place-items-center overflow-hidden"
        >
          <Spinner size="md" />
        </div>
      )}
      {children}
    </div>
  )
})
