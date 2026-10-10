/**
 * Admin query keys. Everything is per workspace, so a workspace switch never
 * shows another workspace's people or roles. `all(wsId)` is the group prefix.
 */
export const adminKeys = {
  all: (wsId: string) => ['admin', wsId] as const,
  departments: (wsId: string) => ['admin', 'departments', wsId] as const,
  /** The people table (name, email, department, roles, standing). */
  members: (wsId: string) => ['admin', 'members', wsId] as const,
  /** Roles: `{ roles, system_roles }`. Approval's role picker reads the same entry. */
  roles: (wsId: string) => ['admin', 'roles', wsId] as const,
  /** Offers to join that are still open. */
  invitations: (wsId: string) => ['admin', 'invitations', wsId] as const,
  /** One role with some of its people and what it holds per area. */
  role: (wsId: string, roleId: string) => ['admin', 'role', wsId, roleId] as const,
  /** Every cached role detail of a workspace. */
  roleDetails: (wsId: string) => ['admin', 'role', wsId] as const,
  /** The areas and, for each, the operations that apply there. */
  permissionAreas: (wsId: string) => ['admin', 'permission-areas', wsId] as const,
}
