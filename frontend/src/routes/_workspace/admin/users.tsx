import { createFileRoute } from '@tanstack/react-router'
import { useState, useMemo, useRef, useEffect } from 'react'
import { useWorkspaces } from '../../../hooks/useWorkspaces'
import { useContacts, type Contact } from '../../../hooks/useContacts'
import { useDepartments } from '../../../hooks/useAdmin'
import { PeekPanel } from '../../../components/composites/PeekPanel'
import { Badge } from '../../../components/primitives/Badge'
import { Avatar } from '../../../components/primitives/Avatar'
import { Heading } from '../../../components/primitives/Heading'
import { LoadingState } from '../../../components/LoadingState'
import { EmptyState } from '../../../components/EmptyState'
import { Users, Search, ChevronRight } from 'lucide-react'

export const Route = createFileRoute('/_workspace/admin/users')({
  component: AdminUsersPage,
})

/** Admin Users — member management table matching Stitch design. */
function AdminUsersPage() {
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = wsParam || wsData?.workspaces?.[0]?.id || ''

  const { data: contactsData, isLoading } = useContacts(wsId)
  const { data: deptData } = useDepartments(wsId)
  const contacts = contactsData?.contacts || []

  const [searchQuery, setSearchQuery] = useState('')
  const [deptFilter, setDeptFilter] = useState('')
  const [selectedUser, setSelectedUser] = useState<Contact | null>(null)

  // Filter contacts
  const filteredContacts = useMemo(() => {
    let result = contacts
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      result = result.filter((c) =>
        c.display_name.toLowerCase().includes(q)
        || c.username.toLowerCase().includes(q)
        || c.email.toLowerCase().includes(q),
      )
    }
    if (deptFilter) {
      result = result.filter((c) => c.department === deptFilter)
    }
    return result
  }, [contacts, searchQuery, deptFilter])

  // Unique departments for filter
  const departments = useMemo(() => {
    const set = new Set<string>()
    for (const c of contacts) {
      if (c.department) set.add(c.department)
    }
    return Array.from(set).sort()
  }, [contacts])

  return (
    <div className="flex-1 flex min-h-0">
      {/* Main content */}
      <div className="flex-1 flex flex-col min-h-0 min-w-0">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-outline-variant
          bg-surface-container-lowest shrink-0">
          <div className="flex items-center gap-3">
            <Heading level={2}>Users</Heading>
            <span className="text-sm text-on-surface-variant">{filteredContacts.length} users</span>
          </div>
        </div>

        {/* Filter bar */}
        <div className="flex items-center gap-3 px-6 py-3 border-b border-outline-variant/30
          bg-surface-container-lowest shrink-0">
          {/* Search */}
          <div className="relative flex-1 max-w-[280px]">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-outline-variant pointer-events-none" />
            <input
              type="text"
              placeholder="Search by name or email..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-2 bg-surface-container-low border border-outline-variant
                rounded-lg text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/10 transition-all"
            />
          </div>

          {/* Department filter — custom dropdown (native <select> ignores CSS on options) */}
          <DeptDropdown
            value={deptFilter}
            departments={departments}
            onChange={setDeptFilter}
          />
        </div>

        {/* Table */}
        {isLoading ? (
          <LoadingState />
        ) : filteredContacts.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <EmptyState
              icon={<Users size={40} className="text-on-surface-variant" strokeWidth={1.5} />}
              title="No users found"
              description={searchQuery || deptFilter ? 'Try adjusting your filters.' : 'Invite members to your workspace.'}
            />
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto">
            {/* Table header */}
            <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-4 px-6 py-2
              bg-surface-container text-label-caps text-on-surface-variant uppercase tracking-wider text-[11px]
              border-b border-outline-variant/30 sticky top-0 z-10">
              <span>Name</span>
              <span>Department</span>
              <span>Email</span>
              <span className="w-20 text-center">Status</span>
            </div>

            {/* Rows */}
            {filteredContacts.map((contact) => (
              <button
                key={contact.user_id}
                onClick={() => setSelectedUser(selectedUser?.user_id === contact.user_id ? null : contact)}
                className={`grid grid-cols-[1fr_1fr_1fr_auto] gap-4 px-6 py-3 w-full text-left
                  border-none cursor-pointer transition-colors border-b border-outline-variant/10
                  ${selectedUser?.user_id === contact.user_id
                    ? 'bg-primary-container/30'
                    : 'bg-transparent hover:bg-surface-container-low'
                  }`}
              >
                {/* Name + Avatar */}
                <div className="flex items-center gap-3 min-w-0">
                  <Avatar name={contact.display_name} src={contact.avatar_url} size="sm" />
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-on-surface truncate">
                      {contact.display_name}
                    </div>
                    <div className="text-xs text-on-surface-variant truncate">
                      @{contact.username}
                    </div>
                  </div>
                </div>

                {/* Department */}
                <div className="flex items-center text-sm text-on-surface-variant truncate">
                  {contact.department || '—'}
                </div>

                {/* Email */}
                <div className="flex items-center text-sm text-on-surface-variant truncate">
                  {contact.email || '—'}
                </div>

                {/* Status */}
                <div className="w-20 flex items-center justify-center">
                  <Badge variant={contact.is_online ? 'primary' : 'secondary'}>
                    {contact.is_online ? 'Online' : 'Offline'}
                  </Badge>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Detail panel */}
      {selectedUser && (
        <PeekPanel title={selectedUser.display_name} onClose={() => setSelectedUser(null)} width={360}>
          <div className="p-4 flex flex-col gap-4">
            {/* Profile */}
            <div className="flex flex-col items-center gap-3 py-4">
              <Avatar name={selectedUser.display_name} src={selectedUser.avatar_url} size="lg" />
              <div className="text-center">
                <div className="text-lg font-semibold text-on-surface">{selectedUser.display_name}</div>
                <div className="text-sm text-on-surface-variant">{selectedUser.email}</div>
              </div>
            </div>

            {/* Info */}
            <div className="flex flex-col gap-3 py-3 border-t border-outline-variant/30">
              <InfoRow label="Username" value={`@${selectedUser.username}`} />
              <InfoRow label="Title" value={selectedUser.title || '—'} />
              <InfoRow label="Department" value={selectedUser.department || '—'} />
              <InfoRow label="Location" value={selectedUser.location || '—'} />
            </div>
          </div>
        </PeekPanel>
      )}
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-sm text-on-surface-variant">{label}</span>
      <span className="text-sm text-on-surface font-medium">{value}</span>
    </div>
  )
}

/** Custom dropdown for department filter — native <select> can't be styled. */
function DeptDropdown({
  value,
  departments,
  onChange,
}: {
  value: string
  departments: string[]
  onChange: (v: string) => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const label = value || 'All Departments'

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 px-3 py-2 border border-outline-variant rounded-lg
          bg-surface text-sm text-on-surface cursor-pointer hover:bg-surface-container-low
          transition-colors outline-none focus:border-primary"
      >
        {label}
        <ChevronRight size={14} className={`transition-transform ${open ? 'rotate-90' : ''}`} />
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1 min-w-[180px] rounded-lg border border-outline-variant
          bg-surface shadow-lg z-50 py-1 max-h-[240px] overflow-y-auto">
          <button
            onClick={() => { onChange(''); setOpen(false) }}
            className={`w-full text-left px-3 py-2 text-sm cursor-pointer transition-colors
              ${!value ? 'bg-primary-container/40 text-primary font-medium' : 'text-on-surface hover:bg-surface-container-low'}`}
          >
            All Departments
          </button>
          {departments.map((d) => (
            <button
              key={d}
              onClick={() => { onChange(d); setOpen(false) }}
              className={`w-full text-left px-3 py-2 text-sm cursor-pointer transition-colors
                ${value === d ? 'bg-primary-container/40 text-primary font-medium' : 'text-on-surface hover:bg-surface-container-low'}`}
            >
              {d}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
