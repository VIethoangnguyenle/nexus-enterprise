import { apiFetch } from './client'

export interface BatchAccessResult {
  results: Record<string, Record<string, boolean>>
}

/**
 * Batch check NGAC permissions for multiple drive objects.
 * Returns a map of objectId → { operation → allowed }.
 */
export function batchCheckAccess(
  objectIds: string[],
  operations: string[] = ['read', 'write', 'delete', 'share'],
): Promise<BatchAccessResult> {
  return apiFetch<BatchAccessResult>('/drive/batch-access', {
    method: 'POST',
    body: JSON.stringify({ object_ids: objectIds, operations }),
  })
}
