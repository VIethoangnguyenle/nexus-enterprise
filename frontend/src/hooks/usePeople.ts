import { useMemo, useSyncExternalStore } from 'react'
import { useContacts } from './useContacts'
import { useWorkspaceTypeOf } from './useWorkspaceType'
import { buildDirectory, EMPTY_DIRECTORY, type PeopleDirectory } from '../lib/people'
import { resolverFor } from '../lib/people-resolver'

/**
 * Everyone the UI can name, indexed for lookups by user id, NGAC node id and username. The
 * directory lists the people the signed-in user may browse; a name it lacks (a hidden or removed
 * person who wrote a message) is fetched in a batch and joins the lookups without joining `list`,
 * so pickers and suggestions never offer anyone the directory hides.
 */
export function usePeople(workspaceId: string): PeopleDirectory & { isLoading: boolean } {
  const { data, isLoading } = useContacts(workspaceId)
  const { isPersonal } = useWorkspaceTypeOf(workspaceId)
  const resolver = resolverFor(workspaceId)
  const resolved = useSyncExternalStore(resolver.subscribe, resolver.getSnapshot)
  const loaded = !!data?.contacts
  return useMemo(() => {
    // Until the directory is known nobody can tell who is missing from it, so nobody is asked about.
    const dir = loaded
      ? buildDirectory(data.contacts, { departments: !isPersonal, resolved, request: resolver.request, requestNode: resolver.requestNode })
      : EMPTY_DIRECTORY
    return { ...dir, isLoading }
  }, [loaded, data, isLoading, isPersonal, resolved, resolver])
}
