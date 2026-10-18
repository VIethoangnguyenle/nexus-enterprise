import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { DriveItem } from '../api/drive'
import type { Channel } from '../api/messaging'

/**
 * The keys of "Đã cùng làm việc". Only the profile reads them; a new group or a new share shows the next time
 * the profile opens (they are kept fresh for a minute, not live).
 */
export const commonWorkKeys = {
  groups: (wsId: string, nodeId: string) => ['contacts', 'common-groups', wsId, nodeId] as const,
  files: (wsId: string, userId: string) => ['contacts', 'shared-by', wsId, userId] as const,
}

/** Who the profile is about: their ids go to the two services and never reach the screen. */
export interface CommonWorkPerson {
  userId: string
  nodeId: string
}

export interface CommonWorkSource<T> {
  items: T[]
  loading: boolean
  failed: boolean
  retry: () => void
}

const STALE_MS = 60_000

/**
 * What the signed-in person has in common with someone, in this workspace (DESIGN.md "Danh bạ" › Điện thoại, thẻ hồ
 * sơ): the groups both are in (messaging) and the files that person owns and shared with them (drive). Each service
 * answers only with what the caller may read and refuses a caller outside the workspace; each source fails on its own.
 */
export function useCommonWork(wsId: string, person: CommonWorkPerson | null): {
  groups: CommonWorkSource<Channel>
  files: CommonWorkSource<DriveItem>
} {
  const on = !!wsId && !!person
  const nodeId = person?.nodeId ?? ''
  const userId = person?.userId ?? ''
  const groups = useQuery({
    queryKey: commonWorkKeys.groups(wsId, nodeId),
    queryFn: () => apiFetch<{ channels: Channel[] | null }>(
      `/workspaces/${encodeURIComponent(wsId)}/channels/in-common?with=${encodeURIComponent(nodeId)}`,
    ),
    enabled: on && !!nodeId,
    staleTime: STALE_MS,
    select: (d) => d.channels ?? [],
  })
  const files = useQuery({
    queryKey: commonWorkKeys.files(wsId, userId),
    queryFn: () => apiFetch<{ items?: DriveItem[] | null }>(
      `/workspaces/${encodeURIComponent(wsId)}/drive/shared-with-me?owner=${encodeURIComponent(userId)}`,
    ),
    enabled: on && !!userId,
    staleTime: STALE_MS,
    select: (d) => d.items ?? [],
  })
  return {
    groups: { items: groups.data ?? [], loading: groups.isLoading, failed: groups.isError, retry: () => void groups.refetch() },
    files: { items: files.data ?? [], loading: files.isLoading, failed: files.isError, retry: () => void files.refetch() },
  }
}
