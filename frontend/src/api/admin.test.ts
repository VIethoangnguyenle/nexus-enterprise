import { beforeEach, describe, expect, it, vi } from 'vitest'

import { adminApi } from './admin'

/**
 * The workspace REST handlers bind JSON into structs and ignore any field they
 * do not know, so a misspelt body is not an error: it is a request that does
 * something else (a rename that blanks the name, a department that is never
 * set). These pin every path and field name to the handler's own structs
 * (backend/services/workspace/internal/rest/admin_handler.go and
 * admin_people_handler.go).
 */
const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))

beforeEach(() => apiFetch.mockReset().mockResolvedValue({}))

const lastCall = () => {
  const [url, init] = apiFetch.mock.calls[apiFetch.mock.calls.length - 1] as [string, RequestInit | undefined]
  return { url: `/api${url}`, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
}

describe('department requests', () => {
  it('create sends name and parent_id, and an empty parent for a root', async () => {
    await adminApi.createDepartment('w', 'Đối soát', 'p')
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/departments', method: 'POST', body: { name: 'Đối soát', parent_id: 'p' } })
    await adminApi.createDepartment('w', 'Gốc')
    expect(lastCall().body).toEqual({ name: 'Gốc', parent_id: '' })
  })

  it('rename sends name; move sends new_parent_id (empty for the root)', async () => {
    await adminApi.updateDepartment('w', 'd', 'Mới')
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/departments/d', method: 'PUT', body: { name: 'Mới' } })
    await adminApi.moveDepartment('w', 'd', '')
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/departments/d/move', method: 'PUT', body: { new_parent_id: '' } })
  })

  it('a person\'s department is department_id, empty to leave', async () => {
    await adminApi.updateMemberDepartment('w', 'n', 'd')
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/members/n/department', method: 'PUT', body: { department_id: 'd' } })
    await adminApi.updateMemberDepartment('w', 'n', '')
    expect(lastCall().body).toEqual({ department_id: '' })
  })
})

describe('people and role requests', () => {
  it('lists the people from the admin directory', async () => {
    await adminApi.listMembers('w')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/admin/members', method: 'GET' })
  })

  it('invites by email with the address and, when chosen, a role and department: never a caller', async () => {
    await adminApi.inviteByEmail('w', { email: 'moi@novapay.vn' })
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/members', method: 'POST', body: { email: 'moi@novapay.vn' } })
    await adminApi.inviteByEmail('w', { email: 'moi@novapay.vn', role_id: 'r', department_id: 'd' })
    expect(lastCall().body).toEqual({ email: 'moi@novapay.vn', role_id: 'r', department_id: 'd' })
  })

  it('lists and revokes the workspace\'s open invitations', async () => {
    await adminApi.listInvitations('w')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/invitations', method: 'GET' })
    await adminApi.revokeInvitation('w', 'i1')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/invitations/i1', method: 'DELETE', body: undefined })
  })

  it('assigns and removes one role through the member\'s own route', async () => {
    await adminApi.assignRole('w', 'n', 'r')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/members/n/roles/r', method: 'PUT', body: undefined })
    await adminApi.unassignRole('w', 'n', 'r')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/members/n/roles/r', method: 'DELETE', body: undefined })
    await adminApi.removeMember('w', 'n')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/members/n', method: 'DELETE' })
  })

  it('creates a role with name, reads and deletes it by its route', async () => {
    await adminApi.createRole('w', 'Kế toán')
    expect(lastCall()).toEqual({ url: '/api/workspaces/w/roles', method: 'POST', body: { name: 'Kế toán' } })
    await adminApi.getRole('w', 'r')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/roles/r', method: 'GET' })
    await adminApi.deleteRole('w', 'r')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/roles/r', method: 'DELETE' })
  })

  it('asks the server which operations fit which area, and sends the whole set for one area', async () => {
    await adminApi.listPermissionAreas('w')
    expect(lastCall()).toMatchObject({ url: '/api/workspaces/w/permission-areas', method: 'GET' })
    await adminApi.setRolePermissions('w', 'r', 'documents', ['read', 'share'])
    expect(lastCall()).toEqual({
      url: '/api/workspaces/w/roles/r/permissions/documents', method: 'PUT', body: { operations: ['read', 'share'] },
    })
    await adminApi.setRolePermissions('w', 'r', 'assets', [])
    expect(lastCall().body).toEqual({ operations: [] })
  })
})
