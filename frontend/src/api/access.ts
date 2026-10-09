import { apiFetch } from './client'

/**
 * The operations the policy service evaluates (backend/ngac/ngac_ops.go).
 * There is no `delete`: trashing, restoring and removing an item are all a
 * `write` on its object attribute, so asking about `delete` returns DENY for
 * everyone and hides the action from users who can perform it.
 */
export const NGAC_OPS = [
  'read',
  'write',
  'upload',
  'approve',
  'share',
  'manage',
  'invite',
  'create_channel',
] as const

export type NgacOp = (typeof NGAC_OPS)[number]

/** What the signed-in user may do with one object: one flag per NGAC operation. */
export type ObjectPerms = Record<NgacOp, boolean>

/** Denied everything: what an object has until the policy service answers. */
export const NO_PERMS: ObjectPerms = Object.fromEntries(NGAC_OPS.map((op) => [op, false])) as ObjectPerms

export interface BatchAccessResult {
  results: Record<string, Record<string, boolean>>
}

/**
 * Batch check NGAC permissions for multiple drive objects.
 * Returns a map of objectId → { operation → allowed }.
 */
export function batchCheckAccess(
  objectIds: string[],
  operations: readonly NgacOp[] = NGAC_OPS,
): Promise<BatchAccessResult> {
  return apiFetch<BatchAccessResult>('/drive/batch-access', {
    method: 'POST',
    body: JSON.stringify({ object_ids: objectIds, operations }),
  })
}
