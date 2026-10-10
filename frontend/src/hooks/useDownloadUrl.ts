import { useCallback } from 'react'
import { queryOptions, useMutation, useQuery } from '@tanstack/react-query'
import { driveApi } from '../api/drive'
import { keys } from './keys'

/**
 * Presigned URLs live 15 minutes (document service). Reusing one for ten is
 * safe for an `<img src>`; anything older is fetched again.
 */
const URL_FRESH_MS = 10 * 60_000

export const downloadUrlQueryOptions = (fileId: string) =>
  queryOptions({
    queryKey: keys.drive.downloadUrl(fileId),
    queryFn: async () => (await driveApi.getDownloadUrl(fileId)).download_url,
    enabled: !!fileId,
    staleTime: URL_FRESH_MS,
  })

/**
 * The URL to show a file from (an image preview). For a file the user asked to
 * save, use `useDownloadFile`, which never reuses a URL.
 */
export function useDownloadUrl(fileId: string | undefined, enabled = true) {
  return useQuery({ ...downloadUrlQueryOptions(fileId ?? ''), enabled: !!fileId && enabled })
}

/**
 * Saves a file: asks the drive for a fresh presigned URL, then clicks a link
 * to it. A failure reaches the user as a toast through the shared mutation
 * handler; callers only need `isDownloading` for a busy state.
 */
export function useDownloadFile() {
  const { mutateAsync, isPending } = useMutation({
    meta: { action: 'tải xuống tệp' },
    mutationFn: async (file: { id: string; name: string }) => {
      const { download_url } = await driveApi.getDownloadUrl(file.id)
      const a = document.createElement('a')
      a.href = download_url
      a.download = file.name
      a.click()
    },
  })

  const download = useCallback(
    // The handler has already told the user; swallow the rejection so a click
    // handler never leaves an unhandled promise behind.
    (file: { id: string; name: string }) => mutateAsync(file).catch(() => undefined),
    [mutateAsync],
  )

  return { download, isDownloading: isPending }
}
