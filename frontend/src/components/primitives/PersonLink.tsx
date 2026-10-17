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
}

/**
 * A person in running content (a message author, an approver, a file's owner) as something to
 * press. A person the signed-in user may browse opens their profile; anyone else opens the
 * hidden-person card, which gives the name and one sentence and nothing to act on. Ids stay out of
 * sight: the button is named by the person's name.
 */
export function PersonLink({ userId, people, onOpenProfile, children, className = '' }: PersonLinkProps) {
  const navigate = useNavigate()
  const anchorRef = useRef<HTMLButtonElement>(null)
  const [cardOpen, setCardOpen] = useState(false)
  const name = displayName(people, userId)
  const person = people.byUserId.get(userId)

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
        onClick={press}
        aria-label={children ? name : undefined}
        aria-busy={state === 'unknown' ? true : undefined}
        aria-haspopup={state === 'hidden' ? 'dialog' : undefined}
        aria-expanded={state === 'hidden' ? cardOpen : undefined}
        className={`inline-flex items-center min-w-0 max-w-full p-0 border-none bg-transparent text-left cursor-pointer
          rounded-md focus-ring ${className}`}
      >
        {children ?? <PersonChip name={name} hueKey={userId} avatarUrl={person?.avatarUrl} />}
      </button>
      <HiddenPersonCard
        open={cardOpen && state === 'hidden'}
        onClose={() => setCardOpen(false)}
        anchorRef={anchorRef}
        name={name}
        hueKey={userId}
        avatarUrl={person?.avatarUrl}
      />
    </>
  )
}
