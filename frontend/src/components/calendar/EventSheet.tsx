import { useRef } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import type { Occurrence } from '../../api/calendar'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { NavSheet } from '../spaces/NavSheet'
import { EventDetailBody, type EventBodyProps } from './EventPopover'

interface Props extends Omit<EventBodyProps, 'occurrence' | 'large'> {
  occurrence: Occurrence | null
  /** `right` is the tablet's sheet (360 wide, 10 off the edge, radius 16); `bottom` the phone's. */
  side: 'bottom' | 'right'
}

function RightSheet({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })
  return createPortal(
    <div className="fixed inset-0 z-modal">
      <motion.div {...m.scrim} className="absolute inset-0 bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.div
        ref={surfaceRef}
        {...m.modal}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="absolute top-2.5 right-2.5 bottom-2.5 w-90 max-w-[calc(100%-1.25rem)] overflow-y-auto rounded-2xl bg-overlay shadow-overlay py-2"
      >
        {children}
      </motion.div>
    </div>,
    document.body,
  )
}

/**
 * The detail of an event on a phone (a bottom sheet, DESIGN.md §5) or a tablet (a sheet on the right): the
 * same content as the desktop popover with bigger touch targets.
 */
export function EventSheet({ occurrence: o, side, onClose, ...rest }: Props) {
  if (!o) return null
  const body = <EventDetailBody occurrence={o} onClose={onClose} large {...rest} />
  return side === 'right'
    ? <RightSheet title={o.title} onClose={onClose}>{body}</RightSheet>
    : <NavSheet label={o.title} onClose={onClose}>{body}</NavSheet>
}
