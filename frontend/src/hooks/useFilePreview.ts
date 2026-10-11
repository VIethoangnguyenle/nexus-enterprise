import { useQuery } from '@tanstack/react-query'
import { driveApi } from '../api/drive'
import { ApiError } from '../api/client'

/** Preview URLs are presigned like downloads (15 min); refetch well before they lapse. */
const PREVIEW_FRESH_MS = 4 * 60_000

/** A refusal or a missing file will not change by asking again. */
const retryPreview = (failures: number, error: unknown) =>
  !(error instanceof ApiError && (error.status === 403 || error.status === 404)) && failures < 1

/**
 * How to show a drive file (`GET /drive/files/{id}/preview`). The request
 * waits while the server converts an Office file for the first time, so
 * `isPending` can last up to ~45 s; the answer is cached per file.
 */
export function useFilePreview(fileId: string | undefined) {
  return useQuery({
    queryKey: ['drive', 'preview', fileId],
    queryFn: () => driveApi.preview(fileId!),
    enabled: !!fileId,
    staleTime: PREVIEW_FRESH_MS,
    retry: retryPreview,
  })
}
