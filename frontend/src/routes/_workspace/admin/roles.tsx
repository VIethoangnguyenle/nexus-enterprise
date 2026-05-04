import { createFileRoute } from '@tanstack/react-router'
import { useState, useCallback } from 'react'
import { useWorkspaces } from '../../../hooks/useWorkspaces'
import { apiFetch } from '../../../api/client'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { PeekPanel } from '../../../components/composites/PeekPanel'
import { Modal } from '../../../components/composites/Modal'
import { ConfirmDialog } from '../../../components/composites/ConfirmDialog'
import { Button } from '../../../components/primitives/Button'
import { Input } from '../../../components/primitives/Input'
import { Badge } from '../../../components/primitives/Badge'
import { Heading } from '../../../components/primitives/Heading'
import { LoadingState } from '../../../components/LoadingState'
import { EmptyState } from '../../../components/EmptyState'
import { Shield, Plus, Pencil, Trash2, Lock, Users } from 'lucide-react'

export const Route = createFileRoute('/_workspace/admin/roles')({
  component: AdminRolesPage,
})

// Types
interface Role {
  id: string
  name: string
  ngac_node_id: string
  is_system?: boolean
  member_count?: number
  description?: string
}

// Well-known system roles
const SYSTEM_ROLES = ['Owners', 'Members']

function isSystemRole(name: string): boolean {
  return SYSTEM_ROLES.some((sr) => name.endsWith(`_${sr}`) || name === sr)
}

/** Admin Roles — role cards with permission management, matching Stitch design. */
function AdminRolesPage() {
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = wsParam || wsData?.workspaces?.[0]?.id || ''
  const qc = useQueryClient()

  // Fetch roles
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'roles', wsId],
    queryFn: () => apiFetch<{ roles: Role[] }>(`/workspaces/${wsId}/roles`),
    enabled: !!wsId,
  })

  const roles = (data?.roles || []).map((r) => ({
    ...r,
    is_system: isSystemRole(r.name),
  }))

  const [selectedRole, setSelectedRole] = useState<Role | null>(null)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [newRoleName, setNewRoleName] = useState('')

  // Create role
  const createMutation = useMutation({
    mutationFn: (name: string) =>
      apiFetch<Role>(`/workspaces/${wsId}/roles`, {
        method: 'POST',
        body: JSON.stringify({ name }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'roles', wsId] })
      setShowCreateModal(false)
      setNewRoleName('')
    },
  })

  const handleCreate = () => {
    if (!newRoleName.trim()) return
    createMutation.mutate(newRoleName.trim())
  }

  const handleDelete = () => {
    if (!selectedRole) return
    // Using existing deleteRole API
    apiFetch(`/workspaces/${wsId}/roles/${selectedRole.id}`, { method: 'DELETE' })
      .then(() => {
        qc.invalidateQueries({ queryKey: ['admin', 'roles', wsId] })
        setSelectedRole(null)
        setShowDeleteConfirm(false)
      })
      .catch(() => setShowDeleteConfirm(false))
  }

  // Partition roles: system first, then custom
  // Deduplicate system roles by display name (NGAC creates multiple UA nodes per PC)
  const systemRolesRaw = roles.filter((r) => r.is_system)
  const seen = new Set<string>()
  const systemRoles = systemRolesRaw.filter((r) => {
    const dn = r.name.split('_').pop() || r.name
    if (seen.has(dn)) return false
    seen.add(dn)
    return true
  })
  const customRoles = roles.filter((r) => !r.is_system)

  // Display-friendly name
  const displayName = (name: string) => {
    // Strip workspace ID prefix (e.g., "uuid_Owners" → "Owners")
    const parts = name.split('_')
    return parts.length > 1 ? parts[parts.length - 1] : name
  }

  return (
    <div className="flex-1 flex min-h-0">
      {/* Main content */}
      <div className="flex-1 flex flex-col min-h-0 min-w-0">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-outline-variant
          bg-surface-container-lowest shrink-0">
          <Heading level={2}>Roles & Permissions</Heading>
          <Button variant="primary" onClick={() => setShowCreateModal(true)}>
            <Plus size={16} />
            New Role
          </Button>
        </div>

        {/* Role cards */}
        {isLoading ? (
          <LoadingState />
        ) : roles.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <EmptyState
              icon={<Shield size={40} className="text-on-surface-variant" strokeWidth={1.5} />}
              title="No roles found"
              description="Roles are created when a workspace is provisioned."
            />
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-3">
            {/* System roles */}
            {systemRoles.length > 0 && (
              <>
                <span className="text-label-caps text-on-surface-variant uppercase tracking-wider text-[11px] font-semibold px-1">
                  System Roles
                </span>
                {systemRoles.map((role) => (
                  <RoleCard
                    key={role.id}
                    role={role}
                    displayName={displayName(role.name)}
                    isSelected={selectedRole?.id === role.id}
                    onSelect={() => setSelectedRole(selectedRole?.id === role.id ? null : role)}
                    isSystem
                  />
                ))}
              </>
            )}

            {/* Custom roles */}
            {customRoles.length > 0 && (
              <>
                <span className="text-label-caps text-on-surface-variant uppercase tracking-wider text-[11px] font-semibold px-1 mt-4">
                  Custom Roles
                </span>
                {customRoles.map((role) => (
                  <RoleCard
                    key={role.id}
                    role={role}
                    displayName={displayName(role.name)}
                    isSelected={selectedRole?.id === role.id}
                    onSelect={() => setSelectedRole(selectedRole?.id === role.id ? null : role)}
                  />
                ))}
              </>
            )}
          </div>
        )}
      </div>

      {/* Detail panel */}
      {selectedRole && (
        <PeekPanel
          title={displayName(selectedRole.name)}
          onClose={() => setSelectedRole(null)}
          width={380}
        >
          <div className="p-4 flex flex-col gap-4">
            {/* Role type */}
            <div className="flex items-center gap-2">
              {selectedRole.is_system ? (
                <Badge variant="secondary">
                  <Lock size={12} />
                  System
                </Badge>
              ) : (
                <Badge variant="primary">Custom</Badge>
              )}
            </div>

            {/* Description */}
            <p className="text-sm text-on-surface-variant">
              {selectedRole.is_system
                ? displayName(selectedRole.name) === 'Owners'
                  ? 'Full access to all workspace resources. Cannot be modified.'
                  : 'Basic access to workspace resources. Cannot be modified.'
                : `Custom role: ${displayName(selectedRole.name)}`}
            </p>

            {/* Permission matrix placeholder */}
            <div className="pt-3 border-t border-outline-variant/30">
              <span className="text-label-caps text-on-surface-variant text-[11px] uppercase tracking-wider">
                Permissions
              </span>
              <div className="mt-3 rounded-lg border border-outline-variant overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-surface-container">
                      <th className="px-3 py-2 text-left text-[11px] text-on-surface-variant uppercase tracking-wider font-medium">Resource</th>
                      <th className="px-2 py-2 text-center text-[11px] text-on-surface-variant uppercase tracking-wider font-medium w-12">R</th>
                      <th className="px-2 py-2 text-center text-[11px] text-on-surface-variant uppercase tracking-wider font-medium w-12">W</th>
                      <th className="px-2 py-2 text-center text-[11px] text-on-surface-variant uppercase tracking-wider font-medium w-12">M</th>
                    </tr>
                  </thead>
                  <tbody>
                    {['Documents', 'Channels', 'Members'].map((resource) => {
                      const isOwner = displayName(selectedRole.name) === 'Owners'
                      return (
                        <tr key={resource} className="border-t border-outline-variant/30">
                          <td className="px-3 py-2 text-on-surface">{resource}</td>
                          <td className="px-2 py-2 text-center">
                            <PermCheck checked={isOwner || resource !== 'Members'} />
                          </td>
                          <td className="px-2 py-2 text-center">
                            <PermCheck checked={isOwner || resource === 'Channels'} />
                          </td>
                          <td className="px-2 py-2 text-center">
                            <PermCheck checked={isOwner} />
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Actions for custom roles */}
            {!selectedRole.is_system && (
              <div className="flex flex-col gap-2 pt-3 border-t border-outline-variant/30">
                <Button
                  variant="ghost"
                  onClick={() => setShowDeleteConfirm(true)}
                  className="justify-start text-error"
                >
                  <Trash2 size={14} />
                  Delete Role
                </Button>
              </div>
            )}
          </div>
        </PeekPanel>
      )}

      {/* Create modal */}
      {showCreateModal && (
        <Modal title="New Role" onClose={() => setShowCreateModal(false)}>
          <div className="flex flex-col gap-4 p-4">
            <div>
              <label className="text-sm font-medium text-on-surface mb-1 block">Role Name</label>
              <Input
                value={newRoleName}
                onChange={(e) => setNewRoleName(e.target.value)}
                placeholder="e.g. Content Manager"
                autoFocus
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="ghost" onClick={() => setShowCreateModal(false)}>Cancel</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                disabled={!newRoleName.trim() || createMutation.isPending}
              >
                {createMutation.isPending ? 'Creating...' : 'Create'}
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* Delete confirm */}
      {showDeleteConfirm && selectedRole && (
        <ConfirmDialog
          title="Delete Role"
          message={`Are you sure you want to delete "${displayName(selectedRole.name)}"? Members assigned to this role will lose its permissions.`}
          confirmLabel="Delete"
          onConfirm={handleDelete}
          onCancel={() => setShowDeleteConfirm(false)}
          variant="danger"
        />
      )}
    </div>
  )
}

// --- PermCheck ---
function PermCheck({ checked }: { checked: boolean }) {
  return (
    <span className={`inline-block w-4 h-4 rounded border ${
      checked
        ? 'bg-primary border-primary text-on-primary'
        : 'bg-transparent border-outline-variant'
    } flex items-center justify-center text-[10px] leading-none`}>
      {checked && '✓'}
    </span>
  )
}

// --- RoleCard ---
interface RoleCardProps {
  role: Role
  displayName: string
  isSelected: boolean
  onSelect: () => void
  isSystem?: boolean
}

function RoleCard({ role, displayName, isSelected, onSelect, isSystem }: RoleCardProps) {
  return (
    <button
      onClick={onSelect}
      className={`flex items-center gap-4 px-4 py-3 rounded-lg border text-left
        w-full cursor-pointer transition-all
        ${isSelected
          ? 'border-primary bg-primary-container/30'
          : 'border-outline-variant/50 bg-surface hover:bg-surface-container-low'
        }`}
    >
      {/* Icon */}
      <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0
        ${isSystem ? 'bg-surface-container-high' : 'bg-primary-container'}`}>
        {isSystem ? (
          <Lock size={18} className="text-on-surface-variant" />
        ) : (
          <Shield size={18} className="text-on-primary-container" />
        )}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-on-surface">{displayName}</span>
          {isSystem && <Badge variant="secondary">System</Badge>}
        </div>
        <div className="text-xs text-on-surface-variant mt-0.5">
          {isSystem
            ? displayName === 'Owners' ? 'Full workspace access' : 'Basic workspace access'
            : 'Custom role'
          }
        </div>
      </div>

      {/* Member count */}
      <div className="flex items-center gap-1 text-xs text-on-surface-variant shrink-0">
        <Users size={12} />
        <span>{role.member_count ?? '—'}</span>
      </div>
    </button>
  )
}
