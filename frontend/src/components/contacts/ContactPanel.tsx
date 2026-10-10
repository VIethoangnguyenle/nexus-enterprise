import { Mail, MessageSquare } from 'lucide-react'
import type { Contact } from '../../hooks/useContacts'
import { Avatar, Button } from '../primitives'
import { SidePanel } from '../spaces/SidePanel'
import { PresenceLabel } from './ContactsTable'
import { nameOf, roleLine } from './contacts-model'

interface ContactPanelProps {
  contact: Contact
  online: boolean
  /** Others in the same department. */
  colleagues: Contact[]
  /** The person opened is the signed-in user: no one to message. */
  isMe: boolean
  messaging: boolean
  onMessage: (c: Contact) => void
  onClose: () => void
}

const MAX_COLLEAGUES = 3

/**
 * A person's profile (mockup §1): name, role, status, the two things you can
 * do (message, email), the details that exist and who works with them. A
 * detail that is empty is left out, not shown as a dash; no id, username or
 * node appears.
 */
export function ContactPanel({ contact, online, colleagues, isMe, messaging, onMessage, onClose }: ContactPanelProps) {
  const name = nameOf(contact)
  const shown = colleagues.slice(0, MAX_COLLEAGUES)
  const others = colleagues.length - shown.length

  const details: [string, string][] = [
    ['Email', contact.email],
    ['Phòng ban', contact.department],
    ['Nơi làm việc', contact.location],
  ].filter((d): d is [string, string] => !!d[1])

  return (
    <SidePanel label="Hồ sơ" title={name} sub={roleLine(contact) || undefined} closeLabel="hồ sơ" onClose={onClose}>
      <div className="grid gap-5 px-3 pb-3">
        <div className="grid justify-items-start gap-3">
          <Avatar name={name} hueKey={contact.user_id} src={contact.avatar_url || undefined} size={64} online={online} />
          <PresenceLabel online={online} />
        </div>

        <div className="flex flex-wrap gap-2">
          {!isMe && (
            <Button size="sm" loading={messaging} onClick={() => onMessage(contact)}>
              <MessageSquare size={16} strokeWidth={1.75} aria-hidden="true" />
              Nhắn tin
            </Button>
          )}
          {contact.email && (
            <a
              href={`mailto:${contact.email}`}
              className="inline-flex items-center gap-2 h-8 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink
                no-underline hover:bg-line focus-ring transition-colors duration-quick"
            >
              <Mail size={16} strokeWidth={1.75} aria-hidden="true" />
              Gửi email
            </a>
          )}
        </div>

        {details.length > 0 && (
          <dl className="m-0 grid gap-3">
            {details.map(([k, v]) => (
              <div key={k} className="grid gap-0.5">
                <dt className="text-xs font-semibold text-ink-muted">{k}</dt>
                <dd className="m-0 text-sm text-ink break-words">{v}</dd>
              </div>
            ))}
          </dl>
        )}

        {shown.length > 0 && (
          <section aria-label="Cùng phòng" className="grid gap-2">
            <h3 className="m-0 text-label text-ink-muted">Cùng phòng</h3>
            <div className="flex items-center gap-2.5">
              <span className="flex items-center">
                {shown.map((c, i) => (
                  <Avatar
                    key={c.user_id}
                    name={nameOf(c)}
                    hueKey={c.user_id}
                    src={c.avatar_url || undefined}
                    size={24}
                    className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`}
                  />
                ))}
              </span>
              <span className="text-sm text-ink-muted min-w-0 truncate">
                {nameOf(shown[0]!)}
                {colleagues.length > 1 ? ` và ${colleagues.length - 1} người khác` : ''}
              </span>
            </div>
            {others > 0 && <span className="sr-only">Còn {others} người nữa</span>}
          </section>
        )}
      </div>
    </SidePanel>
  )
}
