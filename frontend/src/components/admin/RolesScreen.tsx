import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { CircleAlert, KeyRound, Plus } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useRoles } from '../../hooks/useAdmin'
import { editSearch, openSearch, type AdminSearch } from '../../lib/admin-search'
import { useMotionPresets } from '../../lib/motion'
import { Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { AdminFrame } from './AdminFrame'
import { CreateRoleDialog } from './CreateRoleDialog'
import { PermissionEditor } from './PermissionEditor'
import { RolePanel } from './RolePanel'
import { RolesTable } from './RolesTable'

/**
 * Vai trò (design/mockups/admin.html §4 and §5): the two built-in roles and the
 * administrator's own as a table; a role opens in the panel with what it permits
 * by area, and "Sửa quyền" turns the screen into the permission editor. Which
 * role is open, and whether it is being edited, is in the URL.
 */
export function RolesScreen() {
  const search = useSearch({ strict: false }) as AdminSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const { workspaceId: wsId } = useActiveWorkspace()
  const roles = useRoles(wsId)
  const [creating, setCreating] = useState(false)

  const all = useMemo(() => [...(roles.data?.system ?? []), ...(roles.data?.custom ?? [])], [roles.data])
  const go = useCallback(
    (to: (prev: AdminSearch) => AdminSearch) => void navigate({ to: '/admin/roles', search: to }),
    [navigate],
  )
  const openId = search.role ?? null
  const openRole = all.find((r) => r.id === openId)
  const editing = !!search.edit && !!openId
  const closePanel = useCallback(() => go((p) => openSearch(p, 'role')), [go])

  // Esc closes the panel; dialogs claim the key first.
  useEffect(() => {
    if (!openRole || editing) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) closePanel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [openRole, editing, closePanel])

  const loading = roles.isPending
  const failed = roles.isError && !roles.data
  const noCustom = !loading && !failed && (roles.data?.custom.length ?? 0) === 0

  const createButton = (
    <Button size="sm" onClick={() => setCreating(true)}>
      <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
      <span className="max-sm:sr-only">Tạo vai trò</span>
    </Button>
  )

  return (
    <AdminFrame
      section="roles"
      subtitle={roles.data ? `${all.length} vai trò` : undefined}
      action={editing ? undefined : createButton}
    >
      {editing ? (
        <PermissionEditor
          key={openId}
          workspaceId={wsId}
          roleId={openId}
          onDone={() => go((p) => editSearch(p, false))}
        />
      ) : (
        <div className="relative flex flex-1 min-h-0 min-w-0">
          <div className="flex-1 min-w-0 min-h-0 overflow-y-auto">
            {failed ? (
              <EmptyState
                icon={<CircleAlert size={24} strokeWidth={1.75} />}
                text="Không tải được danh sách vai trò. Kiểm tra kết nối rồi thử lại."
                action={<Button variant="soft" size="sm" onClick={() => void roles.refetch()}>Thử lại</Button>}
              />
            ) : (
              <motion.div {...m.route}>
                <div className="pt-2">
                  <RolesTable
                    label="Vai trò"
                    roles={all}
                    loading={loading}
                    openId={openId}
                    onOpen={(r) => go((p) => openSearch(p, 'role', r.id))}
                  />
                </div>
                {noCustom && (
                  <EmptyState
                    compact
                    icon={<KeyRound size={24} strokeWidth={1.75} />}
                    text="Mới có hai vai trò hệ thống. Tạo vai trò như “Kế toán” để cho một nhóm người quyền riêng mà không phải Chủ sở hữu."
                    action={<Button variant="soft" size="sm" onClick={() => setCreating(true)}>Tạo vai trò</Button>}
                  />
                )}
              </motion.div>
            )}
          </div>

          <AnimatePresence>
            {openRole && (
              <RolePanel
                key={openRole.id}
                workspaceId={wsId}
                role={openRole}
                onEdit={() => go((p) => editSearch(p, true))}
                onClose={closePanel}
              />
            )}
          </AnimatePresence>
        </div>
      )}

      <CreateRoleDialog
        open={creating}
        onClose={() => setCreating(false)}
        workspaceId={wsId}
        onCreated={(id) => go((p) => openSearch(p, 'role', id))}
      />
    </AdminFrame>
  )
}
