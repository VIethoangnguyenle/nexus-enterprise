/**
 * Permission query keys, one entry per object.
 *
 * Scoped by tenant so the same object id seen under two tenants occupies two
 * entries: a cache that outlives a tenant switch degrades into a stale read
 * inside the right tenant instead of one tenant reading another's answer.
 */
export const permissionKeys = {
  all: () => ['permissions'] as const,
  object: (tenantId: string | null, objectId: string) =>
    ['permissions', tenantId ?? 'no-tenant', objectId] as const,
}
