import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi, type Department, type DepartmentTree } from '../api/admin'

// --- Query Keys ---
const adminKeys = {
  departments: (wsId: string) => ['admin', 'departments', wsId] as const,
  members: (wsId: string) => ['admin', 'members', wsId] as const,
  roles: (wsId: string) => ['admin', 'roles', wsId] as const,
}

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

// --- Hooks ---

/** Fetch all departments for a workspace, returned as both flat list and tree. */
export function useDepartments(wsId: string) {
  const query = useQuery({
    queryKey: adminKeys.departments(wsId),
    queryFn: () => adminApi.listDepartments(wsId),
    enabled: !!wsId,
    select: (data) => ({
      flat: data.departments || [],
      tree: buildDepartmentTree(data.departments || []),
    }),
  })
  return query
}

/** Create a new department. */
export function useCreateDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, parentId }: { name: string; parentId?: string }) =>
      adminApi.createDepartment(wsId, name, parentId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminKeys.departments(wsId) })
    },
  })
}

/** Rename a department. */
export function useUpdateDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ deptId, name }: { deptId: string; name: string }) =>
      adminApi.updateDepartment(wsId, deptId, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminKeys.departments(wsId) })
    },
  })
}

/** Delete a department. */
export function useDeleteDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (deptId: string) => adminApi.deleteDepartment(wsId, deptId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminKeys.departments(wsId) })
    },
  })
}

/** Move department to new parent. */
export function useMoveDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ deptId, newParentId }: { deptId: string; newParentId: string }) =>
      adminApi.moveDepartment(wsId, deptId, newParentId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminKeys.departments(wsId) })
    },
  })
}

/** Assign member to department. */
export function useUpdateMemberDepartment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, departmentId }: { nodeId: string; departmentId: string }) =>
      adminApi.updateMemberDepartment(wsId, nodeId, departmentId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminKeys.departments(wsId) })
      qc.invalidateQueries({ queryKey: adminKeys.members(wsId) })
    },
  })
}
