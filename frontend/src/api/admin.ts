import { apiFetch } from './client'

// --- Types: what the workspace REST layer returns (internal/rest/admin_*.go) ---

export interface Department {
  id: string
  name: string
  parent_id: string
  member_count: number
}

export interface DepartmentTree extends Department {
  children: DepartmentTree[]
}

/** The two roles every workspace has, and the ones an administrator makes. */
export type RoleKind = 'owners' | 'members' | 'custom'

/** A role in the list. The built-in ones carry no name: the screen words them from `kind`. */
export interface RoleSummary {
  id: string
  name?: string
  ngac_node_id: string
  kind: RoleKind
  member_count: number
}

export interface RoleList {
  /** The administrator's own roles; what pickers elsewhere list. */
  roles: RoleSummary[]
  system_roles: RoleSummary[]
}

/** A person as an avatar and a name. `user_id` is the avatar's colour key, never shown. */
export interface PersonRef {
  ngac_node_id: string
  user_id: string
  display_name: string
  avatar_url: string
}

/** What a role holds on one area: the operations, by NGAC name. */
export interface AreaPermission {
  area: string
  operations: string[]
}

export interface RoleDetail {
  role: RoleSummary
  /** A sample for the avatars; `role.member_count` is the whole. */
  members: PersonRef[]
  permissions: AreaPermission[]
}

/** An area a role can be granted on, with the operations that apply there. */
export interface PermissionArea {
  area: string
  operations: string[]
}

export type MemberStatus = 'active' | 'invited' | 'disabled'

export interface NamedRef {
  id: string
  name: string
}

/** A row of the people table. */
export interface Member {
  ngac_node_id: string
  user_id: string
  display_name: string
  email: string
  avatar_url: string
  title: string
  status: MemberStatus
  is_owner: boolean
  department: NamedRef | null
  roles: NamedRef[]
}

/** An open offer to join, as the workspace's administrators see it. Names, never ids; `id` is only what Thu hồi sends back. */
export interface Invitation {
  id: string
  email: string
  inviter_name: string
  role_name: string
  department_name: string
  created_at: string
  expires_at: string
}

// --- API ---

export const adminApi = {
  // Departments
  listDepartments: (wsId: string) =>
    apiFetch<{ departments: Department[] }>(`/workspaces/${wsId}/departments`),

  createDepartment: (wsId: string, name: string, parentId?: string) =>
    apiFetch<Department>(`/workspaces/${wsId}/departments`, {
      method: 'POST',
      body: JSON.stringify({ name, parent_id: parentId || '' }),
    }),

  updateDepartment: (wsId: string, deptId: string, name: string) =>
    apiFetch<Department>(`/workspaces/${wsId}/departments/${deptId}`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  deleteDepartment: (wsId: string, deptId: string) =>
    apiFetch<void>(`/workspaces/${wsId}/departments/${deptId}`, {
      method: 'DELETE',
    }),

  moveDepartment: (wsId: string, deptId: string, newParentId: string) =>
    apiFetch<Department>(`/workspaces/${wsId}/departments/${deptId}/move`, {
      method: 'PUT',
      body: JSON.stringify({ new_parent_id: newParentId }),
    }),

  /** An empty `departmentId` takes the person out of their department. */
  updateMemberDepartment: (wsId: string, nodeId: string, departmentId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/members/${nodeId}/department`, {
      method: 'PUT',
      body: JSON.stringify({ department_id: departmentId }),
    }),

  // People
  listMembers: (wsId: string) =>
    apiFetch<{ members: Member[] }>(`/workspaces/${wsId}/admin/members`),

  /**
   * Records a pending invitation for the address. The answer is 202
   * `{status: 'invited'}` for any well-formed address and says nothing about
   * whether an account or a membership exists; nobody is added until they accept.
   * 429 when the caller has invited too many addresses for now.
   */
  inviteByEmail: (wsId: string, input: { email: string; role_id?: string; department_id?: string }) =>
    apiFetch<{ status: 'invited' }>(`/workspaces/${wsId}/members`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),

  /** The offers still open in the workspace. */
  listInvitations: (wsId: string) =>
    apiFetch<{ invitations: Invitation[] }>(`/workspaces/${wsId}/invitations`),

  revokeInvitation: (wsId: string, invitationId: string) =>
    apiFetch<void>(`/workspaces/${wsId}/invitations/${invitationId}`, { method: 'DELETE' }),

  removeMember: (wsId: string, nodeId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/members/${nodeId}`, { method: 'DELETE' }),

  assignRole: (wsId: string, nodeId: string, roleId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/members/${nodeId}/roles/${roleId}`, { method: 'PUT' }),

  unassignRole: (wsId: string, nodeId: string, roleId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/members/${nodeId}/roles/${roleId}`, { method: 'DELETE' }),

  // Roles and permissions
  listRoles: (wsId: string) => apiFetch<Partial<RoleList>>(`/workspaces/${wsId}/roles`),

  createRole: (wsId: string, name: string) =>
    apiFetch<RoleSummary>(`/workspaces/${wsId}/roles`, { method: 'POST', body: JSON.stringify({ name }) }),

  getRole: (wsId: string, roleId: string) => apiFetch<RoleDetail>(`/workspaces/${wsId}/roles/${roleId}`),

  deleteRole: (wsId: string, roleId: string) =>
    apiFetch<void>(`/workspaces/${wsId}/roles/${roleId}`, { method: 'DELETE' }),

  /** The areas a role can be granted on and which operations apply on each; the server's table. */
  listPermissionAreas: (wsId: string) =>
    apiFetch<{ areas: PermissionArea[] }>(`/workspaces/${wsId}/permission-areas`),

  /** Makes `operations` exactly what the role holds on `area`; an empty list takes the area away. */
  setRolePermissions: (wsId: string, roleId: string, area: string, operations: string[]) =>
    apiFetch<AreaPermission>(`/workspaces/${wsId}/roles/${roleId}/permissions/${encodeURIComponent(area)}`, {
      method: 'PUT',
      body: JSON.stringify({ operations }),
    }),
}
