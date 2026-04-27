import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { driveApi } from '../api/drive'
import { queryClient } from '../lib/query-client'

// --- Query Options ---

export const driveFolderQueryOptions = (wsId: string, folderId?: string) =>
  queryOptions({
    queryKey: ['drive', wsId, 'folder', folderId || 'root'],
    queryFn: () => folderId ? driveApi.listFolder(folderId) : driveApi.listRoot(wsId),
    enabled: !!wsId,
  })

export const driveItemQueryOptions = (itemId: string) =>
  queryOptions({
    queryKey: ['drive', 'item', itemId],
    queryFn: () => driveApi.getItem(itemId),
    enabled: !!itemId,
  })

export const driveQuotaQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: ['drive', wsId, 'quota'],
    queryFn: () => driveApi.getQuota(wsId),
    enabled: !!wsId,
  })

export const driveSharedWithMeQueryOptions = () =>
  queryOptions({
    queryKey: ['drive', 'shared-with-me'],
    queryFn: () => driveApi.sharedWithMe(),
  })

export const driveSharesQueryOptions = (itemId: string) =>
  queryOptions({
    queryKey: ['drive', 'shares', itemId],
    queryFn: () => driveApi.listShares(itemId),
    enabled: !!itemId,
  })

export const channelDriveQueryOptions = (channelId: string) =>
  queryOptions({
    queryKey: ['drive', 'channel', channelId],
    queryFn: () => driveApi.channelDrive(channelId),
    enabled: !!channelId,
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

export function useChannelDrive(channelId: string) {
  return useQuery(channelDriveQueryOptions(channelId))
}

// --- Mutations ---

export function useCreateFolder(wsId: string) {
  return useMutation({
    mutationFn: ({ name, parentId }: { name: string; parentId?: string }) =>
      driveApi.createFolder(wsId, name, parentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useUploadFile(wsId: string) {
  return useMutation({
    mutationFn: ({ file, parentId }: { file: File; parentId?: string }) =>
      driveApi.upload(wsId, file, parentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useRenameItem(wsId: string) {
  return useMutation({
    mutationFn: ({ itemId, newName }: { itemId: string; newName: string }) =>
      driveApi.renameItem(itemId, newName),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useMoveItem(wsId: string) {
  return useMutation({
    mutationFn: ({ itemId, newParentId }: { itemId: string; newParentId: string }) =>
      driveApi.moveItem(itemId, newParentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useTrashItem(wsId: string) {
  return useMutation({
    mutationFn: (itemId: string) => driveApi.trashItem(itemId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useDeleteItem(wsId: string) {
  return useMutation({
    mutationFn: (itemId: string) => driveApi.deleteItem(itemId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', wsId] }),
  })
}

export function useCreateShare(itemId: string) {
  return useMutation({
    mutationFn: (data: { shareType: string; targetNgacId: string; operations: string[] }) =>
      driveApi.createShare(itemId, data.shareType, data.targetNgacId, data.operations),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', 'shares', itemId] }),
  })
}

export function useRevokeShare(itemId: string) {
  return useMutation({
    mutationFn: (shareId: string) => driveApi.revokeShare(shareId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['drive', 'shares', itemId] }),
  })
}
