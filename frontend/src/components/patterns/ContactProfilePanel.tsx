import type { Contact } from '../../hooks/useContacts'
import { Avatar, Button, IconButton } from '../primitives'
import { X, MessageSquare, Mail, MapPin, Building2, Video } from 'lucide-react'

interface ContactProfilePanelProps {
  contact: Contact
  onClose: () => void
  onMessage?: (userId: string) => void
}

/** Right-panel profile popup matching Stitch contacts-profile.html:
 *  w-96, bg-white border-l border-outline-variant shadow-xl.
 *  Avatar: w-24 h-24 rounded-full border-4 border-white shadow-md with status dot.
 *  Details section: bg-surface-container-low border-y border-outline-variant/30.
 *  Detail rows: w-10 h-10 rounded-full icon containers with shadow-sm.
 *  Action buttons: py-3 rounded-xl with shadow-sm. */
export function ContactProfilePanel({ contact, onClose, onMessage }: ContactProfilePanelProps) {
  return (
    <div className="w-96 border-l border-outline-variant bg-surface-container-lowest shadow-overlay flex flex-col h-full overflow-y-auto">
      {/* Close button — Stitch: absolute top-4 right-4 p-2 rounded-full */}
      <div className="flex justify-end p-4">
        <IconButton onClick={onClose} shape="round" size="lg" aria-label="Đóng">
          <X size={20} />
        </IconButton>
      </div>

      {/* Profile header — Stitch: p-8 pb-6 flex-col items-center text-center */}
      <div className="flex flex-col items-center px-8 pb-6 text-center">
        <div className="relative mb-4">
          <div className="w-24 h-24 rounded-full border-4 border-white shadow-md overflow-hidden">
            <Avatar name={contact.display_name} src={contact.avatar_url} size="lg" />
          </div>
          {/* Status dot — Stitch: absolute bottom-1 right-1 w-5 h-5 */}
          <div className={`absolute bottom-1 right-1 w-5 h-5 rounded-full border-2 border-white
            ${contact.is_online ? 'bg-status-online' : 'bg-status-offline'}`}
          />
        </div>
        <h2 className="text-xl font-bold text-on-surface">{contact.display_name}</h2>
        {contact.title && (
          <p className="text-primary font-medium text-sm">{contact.title}</p>
        )}
        {contact.username && (
          <p className="text-on-surface-variant text-xs mt-1">@{contact.username}</p>
        )}
      </div>

      {/* Details section — Stitch: bg-surface-container-low border-y border-outline-variant/30 px-8 py-6 space-y-4 */}
      <div className="px-8 py-6 bg-surface-container-low border-y border-outline-variant/30 space-y-4">
        {contact.email && (
          <DetailRow icon={<Mail size={20} />} label="Email" value={contact.email} />
        )}
        {contact.department && (
          <DetailRow icon={<Building2 size={20} />} label="Department" value={contact.department} />
        )}
        {contact.location && (
          <DetailRow icon={<MapPin size={20} />} label="Location" value={contact.location} />
        )}
      </div>

      {/* Action buttons — Stitch: p-6 flex-col gap-3 mt-auto */}
      <div className="p-6 flex flex-col gap-3 mt-auto">
        <Button onClick={() => onMessage?.(contact.user_id)} size="cta" className="shadow-sm">
          <MessageSquare size={20} />
          Message
        </Button>
        <Button variant="secondary" size="cta">
          <Video size={20} />
          Call
        </Button>
      </div>
    </div>
  )
}

/** Detail row matching Stitch contacts-profile.html:
 *  Icon in w-10 h-10 rounded-full bg-surface-container-lowest shadow-sm container.
 *  Label: text-xs text-outline uppercase font-bold tracking-wider.
 *  Value: text-sm text-on-surface. */
function DetailRow({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-center gap-4">
      <div className="w-10 h-10 rounded-full bg-surface-container-lowest flex items-center justify-center shadow-sm text-outline shrink-0">
        {icon}
      </div>
      <div className="min-w-0">
        <p className="text-xs text-outline uppercase font-bold tracking-wider">{label}</p>
        <p className="text-sm text-on-surface break-all">{value}</p>
      </div>
    </div>
  )
}
