import {
  ArrowLeft, Building2, CalendarDays, CalendarPlus, ChevronRight, Copy, Mail, MapPin, MessageSquare, MoreHorizontal, Pencil, Phone,
  Shield, UserPlus, X,
} from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Contact } from '../../hooks/useContacts'
import { personStyle } from '../../lib/person-hue'
import { Avatar, IconButton, Pressable, toast } from '../primitives'
import { NavSheet, SheetItem } from '../spaces/NavSheet'
import { OWNERS_LABEL, nameOf, type PathSegment } from './contacts-model'

/**
 * `panel`: the detail column of a desktop. `sheet`: the right sheet of a tablet. `screen`: the full
 * screen of a phone, with a taller cover and the labels above their values. The surface behind it is
 * the caller's; the avatar's ring follows it through `--profile-bg`.
 */
export type ProfileMode = 'panel' | 'sheet' | 'screen'

interface ContactProfileProps {
  contact: Contact
  online: boolean
  /** The signed-in user's own profile: Sửa hồ sơ in place of the three actions. */
  isMe: boolean
  isPersonal: boolean
  /** The department path, root first. */
  path: PathSegment[]
  /** Others in the same department. */
  colleagues: Contact[]
  mode: ProfileMode
  /** A side panel that has just opened with this person takes the focus to their name, so a keyboard reader lands in it. */
  focusHeading?: boolean
  /** What is being started: both actions wait for it. */
  busy: 'message' | 'call' | null
  onMessage: () => void
  onCall: () => void
  onEdit: () => void
  onClose: () => void
  onOpenDept: (id: string) => void
  onOpenRole: (id: string) => void
  /** A phone's Lịch action: Lịch opens with the person's calendar over the viewer's. */
  onCalendar?: () => void
  /** A phone's ⋯ › Thêm vào nhóm…. */
  onAddToGroup?: () => void
  /** A phone's ⋯ › Mời vào cuộc họp. */
  onInviteToMeeting?: () => void
  /** What the screen adds at the end of the card (a phone's "Đã cùng làm việc"). */
  children?: ReactNode
}

const SURFACE: Record<ProfileMode, string> = {
  panel: 'var(--color-raised)', sheet: 'var(--color-overlay)', screen: 'var(--color-base)',
}
const MAX_FACES = 4

const ACTION = `press grid justify-items-center content-center gap-1.5 h-16 rounded-lg border-none text-small-ui font-semibold
  no-underline cursor-pointer focus-ring transition-colors duration-quick disabled:opacity-50 disabled:cursor-not-allowed`
/** A phone's action tile: 16 radius on the raised surface (mockup §12 `.acts4`). */
const PHONE_ACTION = `press grid justify-items-center content-center gap-1.5 h-16 rounded-2xl border-none text-small-ui font-semibold
  cursor-pointer focus-ring transition-colors duration-quick disabled:opacity-50 disabled:cursor-not-allowed`

const mailtoOf = (email: string) => `mailto:${encodeURIComponent(email).replace('%40', '@')}`

/**
 * A person's profile card (DESIGN.md "Danh bạ", mockup §1): a cover in their own hue, avatar 64 over
 * it, name, title and status in words; Nhắn tin, Gọi and Gửi email, or Sửa hồ sơ on one's own; then
 * the details that exist and who works with them. A detail that is empty is left out, not shown as a
 * dash; no id, username or node appears.
 *
 * On a phone (mockup `mobile-shell.html` §12): a 120 cover with ← and ⋯ over it, avatar 80, the name at 22/700, four
 * equal actions Nhắn tin · Gọi · Lịch · Email (the address, to copy or mail), the details grouped with their labels
 * above the values, and ⋯ for Sao chép email · Thêm vào nhóm… · Mời vào cuộc họp.
 */
export function ContactProfile({
  contact, online, isMe, isPersonal, path, colleagues, mode, focusHeading, busy, onMessage, onCall, onEdit, onClose, onOpenDept, onOpenRole,
  onCalendar, onAddToGroup, onInviteToMeeting, children,
}: ContactProfileProps) {
  const name = nameOf(contact)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const [sheet, setSheet] = useState<'email' | 'more' | null>(null)
  useEffect(() => {
    if (focusHeading) headingRef.current?.focus()
  }, [focusHeading, contact.user_id])
  const screen = mode === 'screen'
  const roles = isPersonal ? [] : contact.roles.filter((r) => r.kind !== 'members')
    .map((r) => ({ id: r.id, name: r.kind === 'owners' ? OWNERS_LABEL : r.name }))
  const faces = colleagues.slice(0, MAX_FACES)
  const hasMenu = screen && !isMe && (!!onAddToGroup || !!onInviteToMeeting || !!contact.email)

  const copyEmail = () => {
    const write = navigator.clipboard?.writeText(contact.email)
    if (!write) {
      toast.error('Không sao chép được email')
      return
    }
    write.then(() => toast('Đã sao chép email'), () => toast.error('Không sao chép được email'))
  }

  const rows: Array<{ key: string; icon: ReactNode; label: string; value: ReactNode }> = []
  if (contact.email) {
    rows.push({
      key: 'email', icon: <Mail size={16} strokeWidth={1.75} aria-hidden="true" />, label: 'Email',
      value: (
        <span className="flex items-center gap-1 min-w-0">
          <span className={`min-w-0 ${screen ? 'break-all' : 'truncate'}`} title={contact.email}>{contact.email}</span>
          <IconButton aria-label="Sao chép email" title="Sao chép email" size="sm" onClick={copyEmail} className="shrink-0">
            <Copy size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </span>
      ),
    })
  }
  if (!isPersonal && path.length > 0) {
    rows.push({
      key: 'dept', icon: <Building2 size={16} strokeWidth={1.75} aria-hidden="true" />, label: 'Phòng ban',
      value: (
        <span className="flex flex-wrap items-center gap-x-0.5">
          {path.map((seg, i) => (
            <span key={`${seg.id}-${i}`} className="inline-flex items-center">
              {i > 0 && <ChevronRight size={14} strokeWidth={1.75} aria-hidden="true" className="text-ink-subtle" />}
              <Pressable
                onClick={() => onOpenDept(seg.id)}
                className="p-0 border-none bg-transparent text-left text-ink cursor-pointer rounded-sm focus-ring hover:text-accent hover:underline"
              >
                {seg.name}
              </Pressable>
            </span>
          ))}
        </span>
      ),
    })
  }
  if (roles.length > 0) {
    rows.push({
      key: 'roles', icon: <Shield size={16} strokeWidth={1.75} aria-hidden="true" />, label: 'Vai trò',
      value: (
        <span className="flex flex-wrap items-center gap-x-1">
          {roles.map((r, i) => (
            <span key={r.id}>
              <Pressable
                onClick={() => onOpenRole(r.id)}
                className="p-0 border-none bg-transparent text-left text-ink cursor-pointer rounded-sm focus-ring hover:text-accent hover:underline"
              >
                {r.name}
              </Pressable>
              {i < roles.length - 1 ? ',' : ''}
            </span>
          ))}
        </span>
      ),
    })
  }
  if (contact.location) {
    rows.push({
      key: 'location', icon: <MapPin size={16} strokeWidth={1.75} aria-hidden="true" />, label: 'Nơi làm việc', value: contact.location,
    })
  }

  // --- the actions ---
  const phoneActions = screen && !isMe && onCalendar
  const actions = isMe ? (
    <div className="p-4">
      <Pressable onClick={onEdit} className={`${ACTION} w-full bg-hover text-ink hover:bg-line`}>
        <Pencil size={20} strokeWidth={1.75} aria-hidden="true" />
        Sửa hồ sơ
      </Pressable>
    </div>
  ) : phoneActions ? (
    <div role="group" aria-label="Thao tác" className={`grid gap-2 px-4 pt-3.5 pb-2 ${contact.email ? 'grid-cols-4' : 'grid-cols-3'}`}>
      <Pressable disabled={busy !== null} aria-busy={busy === 'message' || undefined} onClick={onMessage}
        className={`${PHONE_ACTION} bg-accent text-on-accent hover:bg-accent-hover`}>
        <MessageSquare size={20} strokeWidth={1.75} aria-hidden="true" />
        Nhắn tin
      </Pressable>
      <Pressable disabled={busy !== null} aria-busy={busy === 'call' || undefined} onClick={onCall}
        className={`${PHONE_ACTION} bg-raised text-ink hover:bg-hover`}>
        <Phone size={20} strokeWidth={1.75} aria-hidden="true" />
        Gọi
      </Pressable>
      <Pressable onClick={onCalendar} className={`${PHONE_ACTION} bg-raised text-ink hover:bg-hover`}>
        <CalendarDays size={20} strokeWidth={1.75} aria-hidden="true" />
        Lịch
      </Pressable>
      {contact.email && (
        <Pressable onClick={() => setSheet('email')} aria-haspopup="dialog" className={`${PHONE_ACTION} bg-raised text-ink hover:bg-hover`}>
          <Mail size={20} strokeWidth={1.75} aria-hidden="true" />
          Email
        </Pressable>
      )}
    </div>
  ) : (
    <div className={`grid gap-2 p-4 ${contact.email ? 'grid-cols-3' : 'grid-cols-2'}`}>
      <Pressable disabled={busy !== null} aria-busy={busy === 'message' || undefined} onClick={onMessage}
        className={`${ACTION} bg-accent text-on-accent hover:bg-accent-hover`}>
        <MessageSquare size={20} strokeWidth={1.75} aria-hidden="true" />
        Nhắn tin
      </Pressable>
      <Pressable disabled={busy !== null} aria-busy={busy === 'call' || undefined} onClick={onCall}
        className={`${ACTION} bg-hover text-ink hover:bg-line`}>
        <Phone size={20} strokeWidth={1.75} aria-hidden="true" />
        Gọi
      </Pressable>
      {contact.email && (
        <a href={mailtoOf(contact.email)} className={`${ACTION} bg-hover text-ink hover:bg-line`}>
          <Mail size={20} strokeWidth={1.75} aria-hidden="true" />
          Gửi email
        </a>
      )}
    </div>
  )

  return (
    <div className="grid content-start min-w-0" style={{ ['--profile-bg' as string]: SURFACE[mode] }}>
      <div
        className={`relative ${screen ? 'h-30' : 'h-22'}`}
        style={{
          ...personStyle(contact.user_id),
          background: 'linear-gradient(135deg, var(--pw) 0%, var(--pw) 45%, var(--color-accent-wash) 100%)',
        }}
      >
        {screen ? (
          <>
            <IconButton aria-label="Quay lại" title="Quay lại" shape="round" variant="filled" size="lg"
              onClick={onClose} className="absolute top-2 left-2 w-11 h-11 !bg-overlay !text-ink">
              <ArrowLeft size={20} strokeWidth={1.75} aria-hidden="true" />
            </IconButton>
            {hasMenu && (
              <IconButton aria-label="Thêm tuỳ chọn" title="Thêm tuỳ chọn" shape="round" variant="filled" size="lg" aria-haspopup="dialog"
                onClick={() => setSheet('more')} className="absolute top-2 right-2 w-11 h-11 !bg-overlay !text-ink">
                <MoreHorizontal size={20} strokeWidth={1.75} aria-hidden="true" />
              </IconButton>
            )}
          </>
        ) : (
          <IconButton aria-label="Đóng hồ sơ" title="Đóng" onClick={onClose} className="absolute top-2 right-2 !text-ink">
            <X size={18} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        )}
      </div>

      <div className="grid justify-items-start gap-1 px-4">
        <Avatar
          name={name} hueKey={contact.user_id} src={contact.avatar_url || undefined} size={64} online={online}
          className={screen ? '!w-20 !h-20 !text-2xl -mt-10 ring-4 ring-(--profile-bg)' : '-mt-8 ring-4 ring-(--profile-bg)'}
        />
        <h2
          ref={headingRef}
          tabIndex={-1}
          className={`m-0 mt-1.5 outline-none font-display font-bold text-ink ${screen ? 'text-[1.375rem] leading-tight' : 'text-lg'}`}
        >
          {name}
        </h2>
        {contact.title && <span className="text-ink-muted">{contact.title}</span>}
        <span
          className={`mt-1 inline-flex items-center h-5.5 px-2 rounded-full text-xs font-semibold
            ${online ? 'bg-success-wash text-success' : 'bg-sunk text-ink-muted'}`}
        >
          {online ? 'Trực tuyến' : 'Ngoại tuyến'}
        </span>
      </div>

      {actions}

      {rows.length > 0 && (
        <dl className={`m-0 grid ${screen ? 'mx-4 mt-1 mb-2 px-4 rounded-2xl bg-raised' : 'px-4 pt-1 pb-2'}`}>
          {rows.map((r) => (
            <div
              key={r.key}
              className={`grid items-center gap-x-2 border-t border-line first:border-t-0 ${screen
                ? 'grid-cols-1 gap-y-0.5 py-2' : 'grid-cols-[6.5rem_minmax(0,1fr)] min-h-10'}`}
            >
              <dt className="flex items-center gap-2 text-ink-muted [&>svg]:shrink-0">
                {r.icon}
                <span className={screen ? 'text-xs' : ''}>{r.label}</span>
              </dt>
              <dd className={`m-0 min-w-0 [overflow-wrap:anywhere] ${screen ? 'text-16 pl-6' : ''}`}>{r.value}</dd>
            </div>
          ))}
        </dl>
      )}

      {!isPersonal && colleagues.length > 0 && (
        <section aria-label="Cùng phòng" className="grid gap-2 px-4 pt-2 pb-4">
          <h3 className="m-0 text-label-caps text-ink-muted">Cùng phòng</h3>
          <Pressable
            onClick={() => onOpenDept(contact.department_id)}
            className="flex items-center gap-2 p-1.5 -m-1.5 border-none bg-transparent rounded-lg text-left text-ink-muted cursor-pointer
              hover:bg-hover hover:text-ink focus-ring transition-colors duration-quick"
          >
            <span className="flex items-center" aria-hidden="true">
              {faces.map((c, i) => (
                <Avatar key={c.user_id} name={nameOf(c)} hueKey={c.user_id} src={c.avatar_url || undefined} size={24}
                  className={`ring-2 ring-(--profile-bg) ${i > 0 ? '-ml-1.5' : ''}`} />
              ))}
            </span>
            <span className="min-w-0 truncate">
              {nameOf(colleagues[0]!)}
              {colleagues.length > 1 ? ` và ${colleagues.length - 1} người khác` : ''}
            </span>
          </Pressable>
        </section>
      )}

      {children}

      {sheet === 'email' && contact.email && (
        <NavSheet label="Email" onClose={() => setSheet(null)}>
          <p className="m-0 px-3.5 py-2 text-16 text-ink break-all">{contact.email}</p>
          <SheetItem icon={<Copy size={18} strokeWidth={1.75} />} onClick={() => { setSheet(null); copyEmail() }}>Sao chép email</SheetItem>
          <a
            href={mailtoOf(contact.email)}
            onClick={() => setSheet(null)}
            className="flex items-center gap-3.5 w-full min-h-13 px-3.5 rounded-lg text-16 font-medium text-ink no-underline
              hover:bg-hover active:bg-hover transition-colors duration-quick focus-ring"
          >
            <Mail size={18} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
            Gửi email
          </a>
        </NavSheet>
      )}
      {sheet === 'more' && (
        <NavSheet label={name} onClose={() => setSheet(null)}>
          {contact.email && (
            <SheetItem icon={<Copy size={18} strokeWidth={1.75} />} onClick={() => { setSheet(null); copyEmail() }}>Sao chép email</SheetItem>
          )}
          {onAddToGroup && (
            <SheetItem icon={<UserPlus size={18} strokeWidth={1.75} />} onClick={() => { setSheet(null); onAddToGroup() }}>Thêm vào nhóm…</SheetItem>
          )}
          {onInviteToMeeting && (
            <SheetItem icon={<CalendarPlus size={18} strokeWidth={1.75} />} onClick={() => { setSheet(null); onInviteToMeeting() }}>
              Mời vào cuộc họp
            </SheetItem>
          )}
        </NavSheet>
      )}
    </div>
  )
}
