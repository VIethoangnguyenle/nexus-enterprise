import { createFileRoute } from '@tanstack/react-router'
import { useState, useCallback } from 'react'
import { useWorkspaces } from '../../../hooks/useWorkspaces'
import { useDepartments, useCreateDepartment, useUpdateDepartment, useDeleteDepartment } from '../../../hooks/useAdmin'
import { type DepartmentTree } from '../../../api/admin'
import { PeekPanel } from '../../../components/composites/PeekPanel'
import { Modal } from '../../../components/composites/Modal'
import { ConfirmDialog } from '../../../components/composites/ConfirmDialog'
import { Button } from '../../../components/primitives/Button'
import { Input } from '../../../components/primitives/Input'
import { Badge } from '../../../components/primitives/Badge'
import { Heading } from '../../../components/primitives/Heading'
import { LoadingState } from '../../../components/LoadingState'
import { EmptyState } from '../../../components/EmptyState'
import { Building2, ChevronRight, Plus, Pencil, Trash2, Users } from 'lucide-react'

export const Route = createFileRoute('/_workspace/admin/')({
  component: AdminOrganizationPage,
})

/** Admin Organization — Department tree view matching Stitch design. */
function AdminOrganizationPage() {
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = wsParam || wsData?.workspaces?.[0]?.id || ''

  const { data, isLoading } = useDepartments(wsId)
  const createMutation = useCreateDepartment(wsId)
  const updateMutation = useUpdateDepartment(wsId)
  const deleteMutation = useDeleteDepartment(wsId)

  const [selectedDept, setSelectedDept] = useState<DepartmentTree | null>(null)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [editingName, setEditingName] = useState('')
  const [isEditing, setIsEditing] = useState(false)

  // Create modal state
  const [newDeptName, setNewDeptName] = useState('')
  const [newDeptParentId, setNewDeptParentId] = useState('')

  const handleToggle = useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }, [])

  const handleCreate = async () => {
    if (!newDeptName.trim()) return
    await createMutation.mutateAsync({ name: newDeptName.trim(), parentId: newDeptParentId || undefined })
    setNewDeptName('')
    setNewDeptParentId('')
    setShowCreateModal(false)
  }

  const handleRename = async () => {
    if (!selectedDept || !editingName.trim()) return
    await updateMutation.mutateAsync({ deptId: selectedDept.id, name: editingName.trim() })
    setSelectedDept({ ...selectedDept, name: editingName.trim() })
    setIsEditing(false)
  }

  const handleDelete = async () => {
    if (!selectedDept) return
    await deleteMutation.mutateAsync(selectedDept.id)
    setSelectedDept(null)
    setShowDeleteConfirm(false)
  }

  const tree = data?.tree || []

  return (
    <div className="flex-1 flex min-h-0">
      {/* Main content */}
      <div className="flex-1 flex flex-col min-h-0 min-w-0">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-outline-variant
          bg-surface-container-lowest shrink-0">
          <Heading level={2}>Organization Structure</Heading>
          <Button variant="primary" onClick={() => setShowCreateModal(true)}>
            <Plus size={16} />
            New Department
          </Button>
        </div>

        {/* Tree content */}
        {isLoading ? (
          <LoadingState />
        ) : tree.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <EmptyState
              icon={<Building2 size={40} className="text-on-surface-variant" strokeWidth={1.5} />}
              title="No departments yet"
              description="Create your first department to organize your team."
            />
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto p-4">
            {tree.map((node) => (
              <DeptTreeItem
                key={node.id}
                node={node}
                depth={0}
                activeId={selectedDept?.id}
                expandedIds={expandedIds}
                onToggle={handleToggle}
                onSelect={(dept) => {
                  setSelectedDept(dept)
                  setEditingName(dept.name)
                  setIsEditing(false)
                }}
              />
            ))}
          </div>
        )}
      </div>

      {/* Detail panel */}
      {selectedDept && (
        <PeekPanel title={selectedDept.name} onClose={() => setSelectedDept(null)} width={320}>
          <div className="p-4 flex flex-col gap-4">
            {/* Department info */}
            <div className="flex items-center gap-2 text-on-surface-variant">
              <Users size={16} />
              <span className="text-sm">{selectedDept.member_count} members</span>
            </div>

            {/* Edit name */}
            {isEditing ? (
              <div className="flex flex-col gap-2">
                <Input
                  value={editingName}
                  onChange={(e) => setEditingName(e.target.value)}
                  placeholder="Department name"
                  autoFocus
                />
                <div className="flex gap-2">
                  <Button variant="primary" onClick={handleRename} disabled={updateMutation.isPending}>
                    Save
                  </Button>
                  <Button variant="ghost" onClick={() => setIsEditing(false)}>Cancel</Button>
                </div>
              </div>
            ) : null}

            {/* Actions */}
            <div className="flex flex-col gap-2 pt-2 border-t border-outline-variant/30">
              <Button
                variant="ghost"
                onClick={() => { setIsEditing(true); setEditingName(selectedDept.name) }}
                className="justify-start"
              >
                <Pencil size={14} />
                Edit Name
              </Button>
              <Button
                variant="ghost"
                onClick={() => setShowDeleteConfirm(true)}
                className="justify-start text-error"
              >
                <Trash2 size={14} />
                Delete Department
              </Button>
            </div>

            {/* Children info */}
            {selectedDept.children.length > 0 && (
              <div className="pt-2 border-t border-outline-variant/30">
                <span className="text-label-caps text-on-surface-variant text-[11px] uppercase tracking-wider">
                  Sub-departments ({selectedDept.children.length})
                </span>
                <div className="mt-2 flex flex-col gap-1">
                  {selectedDept.children.map((child) => (
                    <button
                      key={child.id}
                      onClick={() => {
                        setSelectedDept(child)
                        setEditingName(child.name)
                        setIsEditing(false)
                      }}
                      className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm text-on-surface
                        hover:bg-surface-container transition-colors border-none bg-transparent cursor-pointer text-left"
                    >
                      <Building2 size={14} className="text-on-surface-variant shrink-0" />
                      <span className="truncate flex-1">{child.name}</span>
                      <Badge variant="secondary">{child.member_count}</Badge>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </PeekPanel>
      )}

      {/* Create modal */}
      {showCreateModal && (
        <Modal title="New Department" onClose={() => setShowCreateModal(false)}>
          <div className="flex flex-col gap-4 p-4">
            <div>
              <label className="text-sm font-medium text-on-surface mb-1 block">Department Name</label>
              <Input
                value={newDeptName}
                onChange={(e) => setNewDeptName(e.target.value)}
                placeholder="e.g. Engineering"
                autoFocus
              />
            </div>
            {data?.flat && data.flat.length > 0 && (
              <div>
                <label className="text-sm font-medium text-on-surface mb-1 block">Parent Department</label>
                <select
                  value={newDeptParentId}
                  onChange={(e) => setNewDeptParentId(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-outline-variant bg-surface
                    text-on-surface text-sm outline-none focus:border-primary transition-colors"
                >
                  <option value="">None (Root department)</option>
                  {data.flat.map((d) => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </select>
              </div>
            )}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="ghost" onClick={() => setShowCreateModal(false)}>Cancel</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                disabled={!newDeptName.trim() || createMutation.isPending}
              >
                {createMutation.isPending ? 'Creating...' : 'Create'}
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* Delete confirm */}
      {showDeleteConfirm && selectedDept && (
        <ConfirmDialog
          title="Delete Department"
          message={`Are you sure you want to delete "${selectedDept.name}"? Sub-departments and members will be reassigned to the parent.`}
          confirmLabel="Delete"
          onConfirm={handleDelete}
          onCancel={() => setShowDeleteConfirm(false)}
          variant="danger"
        />
      )}
    </div>
  )
}

// --- DeptTreeItem ---

interface DeptTreeItemProps {
  node: DepartmentTree
  depth: number
  activeId?: string
  expandedIds: Set<string>
  onToggle: (id: string) => void
  onSelect: (dept: DepartmentTree) => void
}

function DeptTreeItem({ node, depth, activeId, expandedIds, onToggle, onSelect }: DeptTreeItemProps) {
  const isActive = activeId === node.id
  const hasChildren = node.children.length > 0
  const isExpanded = expandedIds.has(node.id)

  return (
    <>
      <button
        onClick={() => {
          onSelect(node)
          if (hasChildren) onToggle(node.id)
        }}
        className={`flex items-center gap-2 w-full text-left px-3 py-2 rounded-lg
          border-none cursor-pointer transition-all text-sm
          ${isActive
            ? 'bg-primary-container/60 text-on-primary-container font-semibold'
            : 'bg-transparent text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
          }`}
        style={{ paddingLeft: `${12 + depth * 20}px` }}
      >
        {/* Chevron */}
        <span className={`flex-shrink-0 transition-transform duration-150 ${isExpanded ? 'rotate-90' : ''}`}>
          {hasChildren ? (
            <ChevronRight size={14} className="text-outline" />
          ) : (
            <span className="w-3.5" />
          )}
        </span>

        {/* Icon */}
        <Building2 size={16} className={isActive ? 'text-primary' : 'text-on-surface-variant'} />

        {/* Label */}
        <span className="truncate flex-1">{node.name}</span>

        {/* Member count */}
        <span className="text-xs text-on-surface-variant shrink-0">
          {node.member_count}
        </span>
      </button>

      {/* Children */}
      {hasChildren && isExpanded && (
        <div className="animate-fade-in">
          {node.children.map((child) => (
            <DeptTreeItem
              key={child.id}
              node={child}
              depth={depth + 1}
              activeId={activeId}
              expandedIds={expandedIds}
              onToggle={onToggle}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </>
  )
}
