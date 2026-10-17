import { useRef, useState, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { HiddenPersonCard } from './HiddenPersonCard'
import { PersonChip } from './PersonChip'

interface PersonLinkProps {
  userId: string
  people: PeopleDirectory
  /** Where a person the directory lists is opened. Defaults to their profile in Danh bạ. */
  onOpenProfile?: (userId: string) => void
  /** What to press: an avatar, a name. Defaults to the person's avatar and name. */
  children?: ReactNode
  className?: string
  /** The name the caller shows (a message's sender, a server-given name); the directory's is the fallback, so label, text and card agree. */
  name?: string
  /** The picture the caller shows; the directory's is the fallback. */
  avatarUrl?: string
  /** A second target for a person whose name is already a link beside it: out of the tab order and the reading order. */
  secondary?: boolean
  /** Id of the text that names this person (name and role), used instead of a label of its own. */
  labelledBy?: string
}

/**
 * A person in running content (a message author, an approver, a file's owner) as something to
 * press. A person the signed-in user may browse opens their profile; anyone else opens the
 * hidden-person card, which gives the name and one sentence and nothing to act on. Ids stay out of
 * sight: the button is named by the person's name.
 */
export function PersonLink({ userId, people, onOpenProfile, children, className = '', name: givenName, avatarUrl: givenAvatar, secondary, labelledBy }: PersonLinkProps) {
  const navigate = useNavigate()
  const anchorRef = useRef<HTMLButtonElement>(null)
  const [cardOpen, setCardOpen] = useState(false)
  const name = givenName?.trim() || displayName(people, userId)
  const person = people.byUserId.get(userId)
  const avatarUrl = givenAvatar || person?.avatarUrl

  // Three states: until the directory is ready nobody can be called hidden, so the press waits.
  const state = !people.ready ? 'unknown' : people.isVisible(userId) ? 'visible' : 'hidden'

  const press = () => {
    if (state === 'unknown') return
    if (state === 'visible') {
      if (onOpenProfile) onOpenProfile(userId)
      // `person` is the Danh bạ route's search key for the open profile; the route declares it with the Danh bạ screens.
      else void navigate({ to: '/contacts', search: (prev: { ws?: string }) => ({ ...(prev.ws ? { ws: prev.ws } : {}), person: userId }) } as never)
      return
    }
    setCardOpen((open) => !open)
  }

  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        onClick={(e) => { e.stopPropagation(); press() }}
        onContextMenu={(e) => e.stopPropagation()}
        tabIndex={secondary ? -1 : undefined}
        aria-hidden={secondary ? true : undefined}
        aria-label={children && !labelledBy ? name : undefined}
        aria-labelledby={labelledBy}
        aria-busy={state === 'unknown' ? true : undefined}
        aria-haspopup={state === 'hidden' ? 'dialog' : undefined}
        aria-expanded={state === 'hidden' ? cardOpen : undefined}
        className={`inline-flex items-center min-w-0 max-w-full p-0 border-none bg-transparent text-left cursor-pointer
          rounded-md focus-ring ${className}`}
      >
        {children ?? <PersonChip name={name} hueKey={userId} avatarUrl={avatarUrl} />}
      </button>
      <HiddenPersonCard
        open={cardOpen && state === 'hidden'}
        onClose={() => setCardOpen(false)}
        anchorRef={anchorRef}
        name={name}
        hueKey={userId}
        avatarUrl={avatarUrl}
      />
    </>
  )
}
