import type { ReactNode } from 'react'
import type { PeopleDirectory } from '../../lib/people'
import { PersonLink } from './PersonLink'

interface LinkedPersonProps {
  /** Set once the directory or the resolver knows the person; without it there is no one to open. */
  userId: string | undefined
  people: PeopleDirectory | undefined
  className?: string
  /** What the caller shows; see `PersonLink`. */
  name?: string
  avatarUrl?: string
  secondary?: boolean
  labelledBy?: string
  children: ReactNode
}

/**
 * A person on an screen as something to press. Approvals know people by NGAC node, so a
 * line only becomes pressable when that node has been matched to a person (`PersonView.userId`):
 * a role, a department or a step nobody holds yet has no profile and stays plain.
 */
export function LinkedPerson({ userId, people, children, ...rest }: LinkedPersonProps) {
  if (!userId || !people) return <>{children}</>
  return <PersonLink userId={userId} people={people} {...rest}>{children}</PersonLink>
}
