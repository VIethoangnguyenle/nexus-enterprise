import type { Contact } from '../../hooks/useContacts'
import { Avatar, Pressable } from '../primitives'
import { PresenceLabel } from './ContactsTable'
import { nameOf, roleLine } from './contacts-model'

/** Danh bạ as cards (mockup §8): the same people, a block each. */
export function ContactCards({ label, people, selectedId, isOnline, onSelect }: {
  label: string
  people: Contact[]
  selectedId: string | null
  isOnline: (c: Contact) => boolean
  onSelect: (c: Contact) => void
}) {
  return (
    <ul aria-label={label} className="m-0 p-0 list-none grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-3 px-5 pb-4">
      {people.map((c) => {
        const name = nameOf(c)
        const online = isOnline(c)
        const selected = selectedId === c.user_id
        return (
          <li key={c.user_id}>
            <Pressable
              aria-current={selected || undefined}
              onClick={() => onSelect(c)}
              className={`w-full grid gap-3 p-4 rounded-surface transition-colors duration-quick
                ${selected ? 'bg-accent-wash' : 'bg-raised hover:bg-hover'}`}
            >
              <span className="flex items-center gap-3 min-w-0">
                <Avatar name={name} hueKey={c.user_id} src={c.avatar_url || undefined} online={online} size={40} />
                <span className="grid min-w-0">
                  <span className="truncate font-semibold text-ink">{name}</span>
                  <span className="truncate text-xs text-ink-muted">{roleLine(c) || c.email}</span>
                </span>
              </span>
              <PresenceLabel online={online} />
            </Pressable>
          </li>
        )
      })}
    </ul>
  )
}
