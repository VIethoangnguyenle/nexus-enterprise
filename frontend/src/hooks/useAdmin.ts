import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminApi,
  type Department,
  type Invitation,
  type DepartmentTree,
  type Member,
  type PermissionArea,
  type RoleDetail,
  type RoleSummary,
} from '../api/admin'
import { statusOf } from '../lib/errors'
import { keys } from './keys'

// --- Department Tree Builder ---
function buildDepartmentTree(departments: Department[]): DepartmentTree[] {
  const map = new Map<string, DepartmentTree>()
  const roots: DepartmentTree[] = []

  for (const dept of departments) {
    map.set(dept.id, { ...dept, children: [] })
  }

  for (const dept of departments) {
    const node = map.get(dept.id)!
    if (dept.parent_id && map.has(dept.parent_id)) {
      map.get(dept.parent_id)!.children.push(node)
    } else {
      roots.push(node)
    }
  }

  return roots
}

/** A 4xx is an answer ("not yours to see", "gone"), not a hiccup: asking again changes nothing. */
const retryUnlessAnswered = (count: number, err: unknown) => {
  const status = statusOf(err)
  return !(status && status >= 400 && status < 500) && count < 1
}

// --- Queries ---

/** Fetch all departments for a workspace, returned as both flat list and tree. */
export function useDepartments(wsId: string) {
  return useQuery({
    queryKey: keys.admin.departments(wsId),
    queryFn: () => adminApi.listDepartments(wsId),
    enabled: !!wsId,
    select: (data) => ({
      flat: data.departments || [],
      tree: buildDepartmentTree(data.departments || []),
    }),
  })
}

/**
 * Everyone in the workspace as a row of the people table. The server answers
 * only a caller who may manage the workspace; anyone else gets a 403, which the
 * screen words as "no permission" rather than as a failure.
 */
export function useMemberDirectory(wsId: string) {
  return useQuery({
    queryKey: keys.admin.members(wsId),
    queryFn: () => adminApi.listMembers(wsId),
    enabled: !!wsId,
    select: (d): Member[] => d.members ?? [],
    retry: retryUnlessAnswered,
  })
}

/** Roles: the built-in two apart from the administrator's own. Approval's role picker reads the same entry. */
export function useRoles(wsId: string, enabled = true) {
  return useQuery({
    queryKey: keys.admin.roles(wsId),
    queryFn: () => adminApi.listRoles(wsId),
    enabled: !!wsId && enabled,
    select: (d): { custom: RoleSummary[]; system: RoleSummary[] } => ({
      custom: d.roles ?? [],
      system: d.system_roles ?? [],
    }),
  })
}

/** One role with some of its people and what it holds per area. */
export function useRole(wsId: string, roleId: string | undefined) {
  return useQuery({
    queryKey: keys.admin.role(wsId, roleId ?? ''),
    queryFn: () => adminApi.getRole(wsId, roleId!),
    enabled: !!wsId && !!roleId,
    retry: retryUnlessAnswered,
  })
}

/** The areas a role can be granted on and, for each, the operations that apply: the server's table. */
export function usePermissionAreas(wsId: string, enabled = true) {
  return useQuery({
    queryKey: keys.admin.permissionAreas(wsId),
    queryFn: () => adminApi.listPermissionAreas(wsId),
    enabled: !!wsId && enabled,
    select: (d): PermissionArea[] => d.areas ?? [],
    // What applies where does not change while the app is open.
    staleTime: 5 * 60_000,
  })
}

// --- Mutations ---

/** After anything that moves people between departments or roles, every table that counts them is stale. */
function useRefreshPeople(wsId: string) {
  const qc = useQueryClient()
  // Fired and not awaited: the caller's own follow-up (a toast, closing a panel
  // whose subject has just gone) must not wait for, or lose to, the refetch.
  return () => {
    void qc.invalidateQueries({ queryKey: keys.admin.members(wsId) })
    void qc.invalidateQueries({ queryKey: keys.admin.roles(wsId) })
    void qc.invalidateQueries({ queryKey: keys.admin.roleDetails(wsId) })
    void qc.invalidateQueries({ queryKey: keys.admin.departments(wsId) })
  }
}

/** Create a new department. */
export function useCreateDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { action: 'tạo phòng ban' },
    mutationFn: ({ name, parentId }: { name: string; parentId?: string }) =>
      adminApi.createDepartment(wsId, name, parentId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.admin.departments(wsId) }),
  })
}

/** Rename a department. */
export function useUpdateDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { action: 'đổi tên phòng ban' },
    mutationFn: ({ deptId, name }: { deptId: string; name: string }) =>
      adminApi.updateDepartment(wsId, deptId, name),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.admin.departments(wsId) })
      void qc.invalidateQueries({ queryKey: keys.admin.members(wsId) })
    },
  })
}

/** Delete a department. */
export function useDeleteDepartment(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'xoá phòng ban' },
    mutationFn: (deptId: string) => adminApi.deleteDepartment(wsId, deptId),
    onSuccess: () => refresh(),
  })
}

/** Move department to new parent. */
export function useMoveDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { action: 'chuyển phòng ban' },
    mutationFn: ({ deptId, newParentId }: { deptId: string; newParentId: string }) =>
      adminApi.moveDepartment(wsId, deptId, newParentId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.admin.departments(wsId) }),
  })
}

/** Put a member in a department, or with an empty id take them out of theirs. */
export function useUpdateMemberDepartment(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'đổi phòng ban của thành viên' },
    mutationFn: ({ nodeId, departmentId }: { nodeId: string; departmentId: string }) =>
      adminApi.updateMemberDepartment(wsId, nodeId, departmentId),
    onSuccess: () => refresh(),
  })
}

/** Give a member one more role. */
export function useAssignRole(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'gán vai trò' },
    mutationFn: ({ nodeId, roleId }: { nodeId: string; roleId: string }) => adminApi.assignRole(wsId, nodeId, roleId),
    onSuccess: () => refresh(),
  })
}

/** Take one role from a member. */
export function useUnassignRole(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'bỏ vai trò' },
    mutationFn: ({ nodeId, roleId }: { nodeId: string; roleId: string }) => adminApi.unassignRole(wsId, nodeId, roleId),
    onSuccess: () => refresh(),
  })
}

/** Remove a member from the workspace. */
export function useRemoveMember(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'xoá thành viên khỏi workspace' },
    mutationFn: (nodeId: string) => adminApi.removeMember(wsId, nodeId),
    onSuccess: () => refresh(),
  })
}

export type InviteStatus = 'invited' | 'invalid' | 'forbidden' | 'rate_limited' | 'failed'

/** What happened to one address. Every well-formed address is "invited": the answer never says more. */
export interface InviteOutcome {
  email: string
  status: InviteStatus
}

const inviteStatusOf = (err: unknown): InviteStatus => {
  switch (statusOf(err)) {
    case 400: return 'invalid'
    case 403: return 'forbidden'
    case 429: return 'rate_limited'
    default: return 'failed'
  }
}

/**
 * Invite several addresses, one after another, each with the same role and
 * department if given. An invitation is an offer the person answers later, so
 * the server's answer is the same for any well-formed address; only an address
 * that is not one, a refusal, or the caller's hourly budget differ. Nothing
 * throws, so the shared error toast stays quiet and the dialog words each
 * address itself. A rate limit stops the rest rather than burning requests.
 */
export function useInvitePeople(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { silentError: true },
    mutationFn: async (input: { emails: string[]; roleId?: string; departmentId?: string }): Promise<InviteOutcome[]> => {
      const out: InviteOutcome[] = []
      for (const email of input.emails) {
        try {
          await adminApi.inviteByEmail(wsId, { email, role_id: input.roleId, department_id: input.departmentId })
          out.push({ email, status: 'invited' })
        } catch (err) {
          const status = inviteStatusOf(err)
          out.push({ email, status })
          if (status === 'rate_limited') {
            for (const rest of input.emails.slice(out.length)) out.push({ email: rest, status })
            break
          }
        }
      }
      return out
    },
    onSettled: () => void qc.invalidateQueries({ queryKey: keys.admin.invitations(wsId) }),
  })
}

/** The offers still open in the workspace. Needs the invite right; a 403 is an answer. */
export function useInvitations(wsId: string) {
  return useQuery({
    queryKey: keys.admin.invitations(wsId),
    queryFn: () => adminApi.listInvitations(wsId),
    enabled: !!wsId,
    select: (d): Invitation[] => d.invitations ?? [],
    retry: retryUnlessAnswered,
  })
}

/** Withdraw an open offer. */
export function useRevokeInvitation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { action: 'thu hồi lời mời' },
    mutationFn: (invitationId: string) => adminApi.revokeInvitation(wsId, invitationId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.admin.invitations(wsId) }),
  })
}

/** Create a role. The dialog says what is wrong with a name inline (reserved, empty), so errors stay quiet. */
export function useCreateRole(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { silentError: true },
    mutationFn: (name: string) => adminApi.createRole(wsId, name),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.admin.roles(wsId) }),
  })
}

/** Delete a role; the people who held it lose what it conferred. */
export function useDeleteRole(wsId: string) {
  const refresh = useRefreshPeople(wsId)
  return useMutation({
    meta: { action: 'xoá vai trò' },
    mutationFn: (roleId: string) => adminApi.deleteRole(wsId, roleId),
    onSuccess: () => refresh(),
  })
}

/**
 * Save a role's permissions: one request per area that changed, each making the
 * area's operations exactly what is sent. All are attempted; the first failure
 * is thrown after the others have run, so a partial save is reported, not hidden.
 * The role is read again either way, so the editor shows what the server holds.
 */
export function useSaveRolePermissions(wsId: string, roleId: string) {
  const qc = useQueryClient()
  return useMutation({
    meta: { action: 'lưu quyền của vai trò' },
    mutationFn: async (areas: { area: string; operations: string[] }[]) => {
      const results = await Promise.allSettled(
        areas.map((a) => adminApi.setRolePermissions(wsId, roleId, a.area, a.operations)),
      )
      const failed = results.find((r): r is PromiseRejectedResult => r.status === 'rejected')
      if (failed) throw failed.reason
    },
    // What was saved is on screen at once; the refetch below only confirms it.
    onSuccess: (_done, areas) =>
      qc.setQueryData<RoleDetail>(keys.admin.role(wsId, roleId), (cur) => {
        if (!cur) return cur
        const changed = new Set(areas.map((a) => a.area))
        const kept = cur.permissions.filter((p) => !changed.has(p.area))
        const set = areas.filter((a) => a.operations.length > 0).map((a) => ({ area: a.area, operations: a.operations }))
        return { ...cur, permissions: [...kept, ...set] }
      }),
    onSettled: () => void qc.invalidateQueries({ queryKey: keys.admin.role(wsId, roleId) }),
  })
}
