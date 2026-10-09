import { useMemo } from 'react'
import { useContacts } from './useContacts'
import { buildDirectory, EMPTY_DIRECTORY, type PeopleDirectory } from '../lib/people'

/**
 * Everyone in the workspace, indexed for lookups by user id, NGAC node id and
 * username. One contacts request backs every name and avatar in the chat
 * area, so nothing falls back to showing an id.
 */
export function usePeople(workspaceId: string): PeopleDirectory & { isLoading: boolean } {
  const { data, isLoading } = useContacts(workspaceId)
  return useMemo(() => {
    const dir = data?.contacts ? buildDirectory(data.contacts) : EMPTY_DIRECTORY
    return { ...dir, isLoading }
  }, [data, isLoading])
}
