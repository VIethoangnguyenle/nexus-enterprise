import type { ChannelMember } from '../../../api/messaging'
import { displayName, type PeopleDirectory, type Person } from '../../../lib/people'

/** A roster entry as a Person: the directory's entry when known, otherwise built from the roster's own name. */
export function memberAsPerson(people: PeopleDirectory, mem: ChannelMember): Person {
  return (
    people.byUserId.get(mem.user_id) ??
    people.byNodeId.get(mem.ngac_node_id) ?? {
      userId: mem.user_id,
      nodeId: mem.ngac_node_id,
      username: mem.username,
      name: displayName(people, mem.user_id, mem.username),
      role: '',
      avatarUrl: '',
    }
  )
}

/** The roster as people, sorted by name the Vietnamese way. */
export function membersAsPeople(people: PeopleDirectory, members: ChannelMember[] | null | undefined): Person[] {
  return (members ?? [])
    .map((m) => memberAsPerson(people, m))
    .sort((a, b) => a.name.localeCompare(b.name, 'vi'))
}
