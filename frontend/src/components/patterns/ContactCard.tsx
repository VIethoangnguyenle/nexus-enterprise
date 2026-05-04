import type { Contact } from '../../hooks/useContacts'
import { Avatar } from '../primitives'
import { MessageSquare, Mail } from 'lucide-react'

interface ContactCardProps {
  contact: Contact
  onSelect: (contact: Contact) => void
  onMessage?: (userId: string) => void
}

/** Contact card — avatar, name, title, department/location pills, Message + email actions.
 *  Used in the grid layout matching Stitch contacts-final design. */
export function ContactCard({ contact, onSelect, onMessage }: ContactCardProps) {
  return (
    <div
      className="flex flex-col items-center bg-surface-container-lowest rounded-xl
        border border-outline-variant/30 p-5 hover:shadow-md hover:-translate-y-0.5
        transition-all cursor-pointer"
      onClick={() => onSelect(contact)}
    >
      {/* Avatar with online indicator */}
      <div className="relative mb-3">
        <div className="w-16 h-16">
          <Avatar name={contact.display_name} src={contact.avatar_url} size="lg" />
        </div>
        {contact.is_online && (
          <span className="absolute bottom-0 right-0 w-3.5 h-3.5 bg-status-online rounded-full
            border-2 border-surface-container-lowest" />
        )}
      </div>

      {/* Name + title */}
      <h4 className="text-body-strong text-on-surface text-center mb-0.5 truncate w-full">
        {contact.display_name}
      </h4>
      {contact.title && (
        <p className="text-caption text-on-surface-variant text-center mb-3 truncate w-full m-0">
          {contact.title}
        </p>
      )}

      {/* Department + Location pills */}
      <div className="flex items-center justify-center gap-1.5 flex-wrap mb-4">
        {contact.department && (
          <span className="inline-flex items-center px-2 py-0.5 rounded-md
            bg-surface-container text-micro text-on-surface-variant font-medium">
            {contact.department}
          </span>
        )}
        {contact.location && (
          <span className="inline-flex items-center px-2 py-0.5 rounded-md
            bg-surface-container text-micro text-on-surface-variant font-medium">
            {contact.location}
          </span>
        )}
      </div>

      {/* Action buttons */}
      <div className="flex items-center gap-2 w-full">
        <button
          onClick={(e) => { e.stopPropagation(); onMessage?.(contact.user_id) }}
          className="flex-1 flex items-center justify-center gap-1.5 py-2 bg-primary hover:bg-primary-hover
            text-on-primary font-semibold text-small rounded-lg transition-colors cursor-pointer border-none"
        >
          <MessageSquare size={14} />
          Message
        </button>
        <button
          onClick={(e) => e.stopPropagation()}
          className="w-9 h-9 flex items-center justify-center rounded-lg
            bg-surface-container-low border border-outline-variant/50
            text-on-surface-variant hover:text-on-surface hover:bg-surface-container
            transition-colors cursor-pointer"
        >
          <Mail size={16} />
        </button>
      </div>
    </div>
  )
}
