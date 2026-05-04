import { apiFetch } from './client'

// --- Types ---

export interface Department {
  id: string
  name: string
  parent_id: string
  member_count: number
}

export interface DepartmentTree extends Department {
  children: DepartmentTree[]
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

  updateMemberDepartment: (wsId: string, nodeId: string, departmentId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/members/${nodeId}/department`, {
      method: 'PUT',
      body: JSON.stringify({ department_id: departmentId }),
    }),
}
