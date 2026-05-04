import type { Contact } from '../../hooks/useContacts'
import { Avatar } from '../primitives'

type StatusType = 'online' | 'offline' | 'meeting'

interface ContactsTableProps {
  contacts: Contact[]
  selectedId?: string
  onSelect: (contact: Contact) => void
}

/** Contacts table matching Stitch contacts-directory.html:
 *  Wrapped in bg-surface-container-lowest rounded-xl shadow-[0_4px_16px] border border-outline-variant.
 *  Table header: CSS Grid grid-cols-[auto_1.5fr_1fr_1.5fr_1fr], bg-surface-container-low, text-xs font-bold uppercase.
 *  Rows: px-6 py-3.5, hover:bg-surface-bright, w-10 h-10 avatars with status dots.
 *  Status badges: px-2 py-0.5 rounded (not rounded-full), border-color matches status. */
export function ContactsTable({ contacts, selectedId, onSelect }: ContactsTableProps) {
  return (
    <div className="flex-1 overflow-y-auto p-5 md:p-8 bg-background">
      <div className="bg-surface-container-lowest rounded-xl shadow-[0_4px_16px_rgba(0,0,0,0.04)] border border-outline-variant overflow-hidden">
        {/* Table header — Stitch: CSS Grid, bg-surface-container-low */}
        <div className="hidden md:grid grid-cols-[auto_1.5fr_1fr_1.5fr_1fr] gap-4 px-6 py-3 bg-surface-container-low
          border-b border-outline-variant text-xs font-bold text-on-surface-variant uppercase tracking-wider">
          <div className="w-10" />
          <div>Name</div>
          <div>Role</div>
          <div>Email</div>
          <div>Status</div>
        </div>

        {/* Mobile header */}
        <div className="md:hidden flex items-center px-4 py-2.5 bg-surface-container-low border-b border-outline-variant
          text-xs font-bold text-on-surface-variant uppercase tracking-wider">
          <div className="flex-1">Name</div>
          <div className="w-20 text-right">Status</div>
        </div>

        {/* Table body — Stitch: divide-y divide-outline-variant */}
        <div className="divide-y divide-outline-variant">
          {contacts.map((contact) => {
            const status = deriveStatus(contact)
            const isSelected = selectedId === contact.user_id

            return (
              <button
                key={contact.user_id}
                onClick={() => onSelect(contact)}
                className={`w-full border-none cursor-pointer transition-colors text-left
                  ${isSelected ? 'bg-primary-fixed/50' : 'bg-transparent hover:bg-surface-bright'}`}
              >
                {/* Desktop: CSS Grid row — Stitch: px-6 py-3.5 items-center */}
                <div className="hidden md:grid grid-cols-[auto_1.5fr_1fr_1.5fr_1fr] gap-4 px-6 py-3.5 items-center">
                  {/* Avatar cell — Stitch: w-10 relative, status dot */}
                  <div className="w-10 relative">
                    <Avatar
                      name={contact.display_name}
                      src={contact.avatar_url}
                      size="md"
                    />
                    <StatusDot status={status} />
                  </div>
                  {/* Name cell — Stitch: font-medium text-on-surface + text-xs text-on-surface-variant */}
                  <div className="flex flex-col min-w-0">
                    <span className="font-medium text-on-surface truncate">{contact.display_name}</span>
                    <span className="text-xs text-on-surface-variant mt-0.5 truncate">@{contact.username}</span>
                  </div>
                  {/* Role cell — Stitch: text-sm text-on-surface-variant */}
                  <div className="text-sm text-on-surface-variant truncate">
                    {contact.title || '—'}
                  </div>
                  {/* Email cell — Stitch: text-sm text-on-surface-variant */}
                  <div className="text-sm text-on-surface-variant truncate">
                    {contact.email || '—'}
                  </div>
                  {/* Status cell */}
                  <div className="flex items-center">
                    <StatusBadge status={status} />
                  </div>
                </div>

                {/* Mobile: Flex row */}
                <div className="md:hidden flex items-center px-4 py-3 gap-3">
                  <div className="relative shrink-0">
                    <Avatar name={contact.display_name} src={contact.avatar_url} size="sm" />
                    <StatusDot status={status} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-on-surface text-sm truncate">{contact.display_name}</div>
                    <div className="text-xs text-on-surface-variant truncate">{contact.title || contact.email}</div>
                  </div>
                  <StatusBadge status={status} />
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function deriveStatus(contact: Contact): StatusType {
  if (contact.is_online) return 'online'
  return 'offline'
}

/** Status dot overlay — Stitch: absolute bottom-0 right-0 w-3 h-3 border-2 border-white rounded-full. */
function StatusDot({ status }: { status: StatusType }) {
  const bg = status === 'online' ? 'bg-status-online'
    : status === 'meeting' ? 'bg-status-meeting'
    : 'bg-status-offline'

  return (
    <div className={`absolute bottom-0 right-0 w-3 h-3 ${bg} border-2 border-white rounded-full`} />
  )
}

/** Status badge matching Stitch: px-2 py-0.5 rounded (not rounded-full),
 *  text-xs font-medium, color-coded with border. Uses semantic status tokens. */
function StatusBadge({ status }: { status: StatusType }) {
  const config = {
    online: { label: 'Online', classes: 'bg-status-online-bg text-status-online border-status-online-border' },
    offline: { label: 'Offline', classes: 'bg-status-offline-bg text-on-surface-variant border-status-offline-border' },
    meeting: { label: 'In meeting', classes: 'bg-status-meeting-bg text-status-meeting border-status-meeting-border' },
  }
  const c = config[status]
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${c.classes}`}>
      {c.label}
    </span>
  )
}
