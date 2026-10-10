import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { driveApi, type DriveItem, type DriveListing, type SharePermission } from '../api/drive'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Query Options ---

export const driveFolderQueryOptions = (wsId: string, folderId?: string) =>
  queryOptions({
    queryKey: keys.drive.folder(wsId, folderId),
    queryFn: () => folderId ? driveApi.listFolder(wsId, folderId) : driveApi.listRoot(wsId),
    enabled: !!wsId,
  })

export const driveItemQueryOptions = (itemId: string) =>
  queryOptions({
    queryKey: keys.drive.item(itemId),
    queryFn: () => driveApi.getItem(itemId),
    enabled: !!itemId,
  })

export const driveQuotaQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: keys.drive.quota(wsId),
    queryFn: () => driveApi.getQuota(wsId),
    enabled: !!wsId,
  })

export const driveSharedWithMeQueryOptions = () =>
  queryOptions({
    queryKey: keys.drive.sharedWithMe(),
    queryFn: () => driveApi.sharedWithMe(),
  })

export const driveSharesQueryOptions = (itemId: string) =>
  queryOptions({
    queryKey: keys.drive.shares(itemId),
    queryFn: async () => (await driveApi.listShares(itemId)).shares ?? [],
    enabled: !!itemId,
  })

export const channelDriveQueryOptions = (wsId: string, channelId: string) =>
  queryOptions({
    queryKey: keys.drive.channel(wsId, channelId),
    queryFn: () => driveApi.channelDrive(wsId, channelId),
    enabled: !!wsId && !!channelId,
  })

// --- Hooks ---

/**
 * Lists folder contents. Pass folderId for subfolders, omit for root. A tree
 * node that is still collapsed passes `enabled = false` so it costs nothing.
 */
export function useDriveFolder(wsId: string, folderId?: string, enabled = true) {
  const options = driveFolderQueryOptions(wsId, folderId)
  return useQuery({ ...options, enabled: !!wsId && enabled })
}

export function useDriveItem(itemId: string) {
  return useQuery(driveItemQueryOptions(itemId))
}

export function useDriveQuota(wsId: string) {
  return useQuery(driveQuotaQueryOptions(wsId))
}

export function useSharedWithMe(enabled = true) {
  return useQuery({ ...driveSharedWithMeQueryOptions(), enabled })
}

/** Who an item is shared with. Only people who may share it can ask, so callers pass `enabled`. */
export function useDriveShares(itemId: string, enabled = true) {
  return useQuery({ ...driveSharesQueryOptions(itemId), enabled: !!itemId && enabled })
}

export function useChannelDrive(wsId: string, channelId: string) {
  return useQuery(channelDriveQueryOptions(wsId, channelId))
}

// --- Mutations ---

export function useCreateFolder(wsId: string) {
  return useMutation({
    meta: { action: 'tạo thư mục' },
    mutationFn: ({ name, parentId }: { name: string; parentId?: string }) =>
      driveApi.createFolder(wsId, name, parentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.all(wsId) }),
  })
}

export function useUploadFile(wsId: string) {
  return useMutation({
    meta: { action: 'tải tệp lên' },
    mutationFn: ({ file, parentId }: { file: File; parentId?: string }) =>
      driveApi.upload(wsId, file, parentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.all(wsId) }),
  })
}

export function useRenameItem(wsId: string) {
  return useMutation({
    meta: { action: 'đổi tên' },
    mutationFn: ({ itemId, newName }: { itemId: string; newName: string }) =>
      driveApi.renameItem(itemId, newName),
    onSuccess: (_d, { itemId }) => invalidateItem(wsId, itemId),
  })
}

export function useMoveItem(wsId: string) {
  return useMutation({
    meta: { action: 'di chuyển' },
    mutationFn: ({ itemId, targetFolderId }: { itemId: string; targetFolderId: string }) =>
      driveApi.moveItem(itemId, targetFolderId),
    onSuccess: (_d, { itemId }) => invalidateItem(wsId, itemId),
  })
}

/**
 * After a change to one item: every workspace listing and quota, plus the two
 * places outside that prefix where the item also shows up.
 */
function invalidateItem(wsId: string, itemId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.drive.all(wsId) }),
    queryClient.invalidateQueries({ queryKey: keys.drive.sharedWithMe() }),
    queryClient.invalidateQueries({ queryKey: keys.drive.item(itemId) }),
  ])
}

/** Optimistic removal helper: removes an item from all drive folder caches. */
function optimisticRemoveItem(wsId: string, itemId: string) {
  const cache = queryClient.getQueryCache()
  const folderQueries = cache.findAll({ queryKey: keys.drive.folders(wsId) })
  const snapshots: Snapshot[] = []

  for (const query of folderQueries) {
    const data = query.state.data as DriveListing | undefined
    if (!data?.items) continue
    snapshots.push({ queryKey: query.queryKey, data })
    queryClient.setQueryData<DriveListing>(query.queryKey, {
      ...data,
      items: data.items.filter((item: DriveItem) => item.id !== itemId),
    })
  }

  const shared = queryClient.getQueryData<DriveListing>(keys.drive.sharedWithMe())
  if (shared?.items) {
    snapshots.push({ queryKey: keys.drive.sharedWithMe(), data: shared })
    queryClient.setQueryData<DriveListing>(keys.drive.sharedWithMe(), {
      ...shared,
      items: shared.items.filter((item: DriveItem) => item.id !== itemId),
    })
  }
  return snapshots
}

type Snapshot = { queryKey: readonly unknown[]; data: DriveListing }

/** Rollback optimistic removal. */
function rollbackRemoveItem(snapshots: Snapshot[]) {
  for (const snap of snapshots) {
    queryClient.setQueryData(snap.queryKey, snap.data)
  }
}

/**
 * Trash an item with optimistic removal from the folder view. Trashed items
 * have no screen of their own yet, so for the user this is "delete"; the
 * caller offers Undo, which restores.
 */
export function useTrashItem(wsId: string) {
  return useMutation({
    meta: { action: 'xoá' },
    mutationFn: (itemId: string) => driveApi.trashItem(itemId),
    onMutate: async (itemId) => {
      await Promise.all([
        queryClient.cancelQueries({ queryKey: keys.drive.folders(wsId) }),
        queryClient.cancelQueries({ queryKey: keys.drive.sharedWithMe() }),
      ])
      return { snapshots: optimisticRemoveItem(wsId, itemId) }
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshots) rollbackRemoveItem(context.snapshots)
    },
    // Quota and the other listings the item showed up in.
    onSettled: (_d, _e, itemId) => invalidateItem(wsId, itemId),
  })
}

/** Bring a trashed item back (the Undo of a delete). */
export function useRestoreItem(wsId: string) {
  return useMutation({
    meta: { action: 'hoàn tác việc xoá' },
    mutationFn: (itemId: string) => driveApi.restoreItem(itemId),
    onSuccess: (_d, itemId) => invalidateItem(wsId, itemId),
  })
}

export function useCreateShare(itemId: string) {
  return useMutation({
    meta: { action: 'chia sẻ' },
    mutationFn: (data: { shareType: 'user'; targetNodeId: string; permission: SharePermission }) =>
      driveApi.createShare(itemId, data.shareType, data.targetNodeId, data.permission),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.shares(itemId) }),
  })
}

/**
 * Change what an existing share allows. The REST layer has no update, so the
 * old share is revoked and a new one created. If the new one cannot be granted
 * the old permission is put back, so a failed change never costs someone access.
 */
export function useChangeSharePermission(itemId: string) {
  return useMutation({
    meta: { action: 'đổi quyền truy cập' },
    mutationFn: async (v: { shareId: string; targetNodeId: string; from: SharePermission; to: SharePermission }) => {
      await driveApi.revokeShare(v.shareId)
      try {
        await driveApi.createShare(itemId, 'user', v.targetNodeId, v.to)
      } catch (err) {
        await driveApi.createShare(itemId, 'user', v.targetNodeId, v.from).catch(() => {})
        throw err
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.drive.shares(itemId) }),
  })
}

export function useRevokeShare(itemId: string) {
  return useMutation({
    meta: { action: 'thu hồi quyền truy cập' },
    mutationFn: (shareId: string) => driveApi.revokeShare(shareId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.shares(itemId) }),
  })
}
