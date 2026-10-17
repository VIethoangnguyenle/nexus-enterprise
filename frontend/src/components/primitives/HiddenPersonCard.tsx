import type { RefObject } from 'react'
import { CircleAlert } from 'lucide-react'
import { usePhone } from '../../hooks/usePhone'
import { NavSheet } from '../spaces/NavSheet'
import { Avatar } from './Avatar'
import { Popover } from './Popover'

/*
 * Not exported from the primitives index: this file reaches NavSheet, which imports that index, so
 * callers import PersonLink and HiddenPersonCard from their own files.
 */

/** The one thing said about a person the signed-in user may no longer open (DESIGN.md, Danh bạ). */
export const HIDDEN_PERSON_SENTENCE = 'Hồ sơ này không còn hiển thị với bạn.'

interface HiddenPersonCardProps {
  open: boolean
  onClose: () => void
  /** The name or avatar that was pressed; the popover hangs from it and focus returns to it. */
  anchorRef: RefObject<HTMLElement | null>
  name: string
  /** Colour key for the avatar (user id); never rendered. */
  hueKey?: string
  avatarUrl?: string
}

/**
 * What a press on a person the directory hides opens: their avatar and name, and the sentence
 * above. No title, department or email, and nothing to press (no Nhắn tin, no Gọi). A popover
 * from 768 up, a bottom sheet below it. Esc and a press outside close it and focus goes back.
 */
export function HiddenPersonCard({ open, onClose, anchorRef, name, hueKey, avatarUrl }: HiddenPersonCardProps) {
  const phone = usePhone()
  if (!open) return null

  const body = (
    <div className="grid gap-2.5 justify-items-start p-2.5 max-w-75">
      <span className="flex items-center gap-2.5 min-w-0 max-w-full">
        <Avatar name={name} hueKey={hueKey} src={avatarUrl} size={40} />
        <b className="truncate text-ink">{name}</b>
      </span>
      <span className="flex items-start gap-2 text-ink-muted">
        <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" className="mt-0.75 shrink-0" />
        <span>{HIDDEN_PERSON_SENTENCE}</span>
      </span>
    </div>
  )

  if (phone) {
    return <NavSheet label={name} onClose={onClose}>{body}</NavSheet>
  }
  return (
    <Popover open onClose={onClose} anchorRef={anchorRef} label={name} className="max-w-75">
      {body}
    </Popover>
  )
}
