import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { driveApi, type DriveItem } from '../api/drive'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Query Options ---

export const driveFolderQueryOptions = (wsId: string, folderId?: string) =>
  queryOptions({
    queryKey: keys.drive.folder(wsId, folderId),
    queryFn: () => folderId ? driveApi.listFolder(folderId) : driveApi.listRoot(wsId),
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
    queryFn: () => driveApi.listShares(itemId),
    enabled: !!itemId,
  })

export const channelDriveQueryOptions = (wsId: string, channelId: string) =>
  queryOptions({
    queryKey: keys.drive.channel(wsId, channelId),
    queryFn: () => driveApi.channelDrive(wsId, channelId),
    enabled: !!wsId && !!channelId,
  })

// --- Hooks ---

/** Lists folder contents. Pass folderId for subfolders, omit for root. */
export function useDriveFolder(wsId: string, folderId?: string) {
  return useQuery(driveFolderQueryOptions(wsId, folderId))
}

export function useDriveItem(itemId: string) {
  return useQuery(driveItemQueryOptions(itemId))
}

export function useDriveQuota(wsId: string) {
  return useQuery(driveQuotaQueryOptions(wsId))
}

export function useSharedWithMe() {
  return useQuery(driveSharedWithMeQueryOptions())
}

export function useDriveShares(itemId: string) {
  return useQuery(driveSharesQueryOptions(itemId))
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
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.all(wsId) }),
  })
}

export function useMoveItem(wsId: string) {
  return useMutation({
    meta: { action: 'di chuyển' },
    mutationFn: ({ itemId, newParentId }: { itemId: string; newParentId: string }) =>
      driveApi.moveItem(itemId, newParentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.all(wsId) }),
  })
}

/** Optimistic removal helper: removes an item from all drive folder caches. */
function optimisticRemoveItem(wsId: string, itemId: string) {
  const cache = queryClient.getQueryCache()
  const folderQueries = cache.findAll({ queryKey: keys.drive.folders(wsId) })
  const snapshots: { queryKey: unknown[]; data: { items: DriveItem[] } }[] = []

  for (const query of folderQueries) {
    const data = query.state.data as { items: DriveItem[] } | undefined
    if (!data?.items) continue
    snapshots.push({ queryKey: query.queryKey, data })
    queryClient.setQueryData(query.queryKey, {
      ...data,
      items: data.items.filter((item: DriveItem) => item.id !== itemId),
    })
  }
  return snapshots
}

/** Rollback optimistic removal. */
function rollbackRemoveItem(snapshots: { queryKey: unknown[]; data: { items: DriveItem[] } }[]) {
  for (const snap of snapshots) {
    queryClient.setQueryData(snap.queryKey, snap.data)
  }
}

/** Trash an item with optimistic removal from folder view. */
export function useTrashItem(wsId: string) {
  return useMutation({
    meta: { action: 'chuyển vào thùng rác' },
    mutationFn: (itemId: string) => driveApi.trashItem(itemId),
    onMutate: async (itemId) => {
      await queryClient.cancelQueries({ queryKey: keys.drive.folders(wsId) })
      return { snapshots: optimisticRemoveItem(wsId, itemId) }
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshots) rollbackRemoveItem(context.snapshots)
    },
  })
}

/** Permanently delete an item with optimistic removal from folder view. */
export function useDeleteItem(wsId: string) {
  return useMutation({
    meta: { action: 'xoá vĩnh viễn' },
    mutationFn: (itemId: string) => driveApi.deleteItem(itemId),
    onMutate: async (itemId) => {
      await queryClient.cancelQueries({ queryKey: keys.drive.folders(wsId) })
      return { snapshots: optimisticRemoveItem(wsId, itemId) }
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshots) rollbackRemoveItem(context.snapshots)
    },
  })
}

export function useCreateShare(itemId: string) {
  return useMutation({
    mutationFn: (data: { shareType: string; targetNgacId: string; operations: string[] }) =>
      driveApi.createShare(itemId, data.shareType, data.targetNgacId, data.operations),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.shares(itemId) }),
  })
}

export function useRevokeShare(itemId: string) {
  return useMutation({
    mutationFn: (shareId: string) => driveApi.revokeShare(shareId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.drive.shares(itemId) }),
  })
}
