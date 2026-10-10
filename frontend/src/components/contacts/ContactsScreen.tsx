import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { CircleAlert, LayoutGrid, List, Users } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConnectionLost } from '../../hooks/useConnectionLost'
import { useContacts, type Contact } from '../../hooks/useContacts'
import { useCreateDM } from '../../hooks/useMessaging'
import { usePreferences } from '../../hooks/usePreferences'
import { explain } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Button, FilterChip, Heading, Pressable, SearchField, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { ContactCards } from './ContactCards'
import { ContactPanel } from './ContactPanel'
import { ContactsTable } from './ContactsTable'
import { DepartmentFilter } from './DepartmentFilter'
import {
  NO_FILTER, colleaguesOf, departmentsOf, filterContacts, nameOf, sortContacts, type ContactFilter,
} from './contacts-model'

/**
 * Danh bạ (design/mockups/contacts-documents-settings.html §1): everyone in the
 * workspace as a hairline table or cards, filtered by name, department and who
 * is online; picking a person opens their profile beside the list. There is no
 * list panel on this screen, so the department filter sits in the toolbar.
 */
export function ContactsScreen() {
  const navigate = useNavigate()
  const m = useMotionPresets()
  const connectionLost = useConnectionLost()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const q = useContacts(wsId)
  const me = useAuthStore((s) => s.user)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const createDM = useCreateDM()
  const { prefs, update } = usePreferences()

  const [filter, setFilter] = useState<ContactFilter>(NO_FILTER)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const all = useMemo(() => sortContacts(q.data?.contacts ?? []), [q.data])
  const isOnline = useCallback(
    (c: Contact) => c.user_id === me?.id || c.user_id in onlineUsers,
    [me?.id, onlineUsers],
  )
  const people = useMemo(() => filterContacts(all, filter, isOnline), [all, filter, isOnline])
  const departments = useMemo(() => departmentsOf(all), [all])
  const selected = selectedId ? all.find((c) => c.user_id === selectedId) : undefined
  const loading = !q.data && !q.isError
  const workspace = workspaceDisplayName(workspaceName)
  const scope = `${wsId}:${filter.department}:${filter.onlineOnly}:${filter.query}`

  // A workspace switch is a different directory.
  useEffect(() => {
    setSelectedId(null)
    setFilter(NO_FILTER)
  }, [wsId])

  // Esc closes the profile; menus and dialogs claim the key first on `document`.
  useEffect(() => {
    if (!selected) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) setSelectedId(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selected])

  const message = (c: Contact) => {
    createDM.mutate(
      { userId: c.user_id, ngacNodeId: c.ngac_node_id },
      {
        onSuccess: (ch) => void navigate({ to: '/channels/$channelId', params: { channelId: ch.id } }),
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${nameOf(c)}`)),
      },
    )
  }

  const body = (() => {
    if (q.isError) {
      return (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được danh bạ. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>}
        />
      )
    }
    if (!loading && all.length === 0) {
      return (
        <EmptyState
          icon={<Users size={24} strokeWidth={1.75} />}
          text="Workspace này chưa có ai khác. Mời đồng nghiệp ở Quản trị để họ hiện ở đây."
        />
      )
    }
    if (!loading && people.length === 0) {
      return (
        <EmptyState
          icon={<Users size={24} strokeWidth={1.75} />}
          text={
            filter.query.trim()
              ? `Không có ai khớp “${filter.query.trim()}”${filter.department ? ` trong ${filter.department}` : ''}. Thử một cách viết khác hoặc xoá bộ lọc.`
              : 'Không có ai khớp bộ lọc này.'
          }
          action={<Button variant="soft" size="sm" onClick={() => setFilter(NO_FILTER)}>Xoá bộ lọc</Button>}
        />
      )
    }
    const label = `Người trong ${workspace}`
    return (
      <motion.div key={prefs.contactsView} {...m.route}>
        {prefs.contactsView === 'cards' && !loading ? (
          <ContactCards label={label} people={people} selectedId={selectedId} isOnline={isOnline} onSelect={(c) => setSelectedId(c.user_id)} />
        ) : (
          <ContactsTable
            label={label}
            people={people}
            loading={loading}
            selectedId={selectedId}
            isOnline={isOnline}
            onSelect={(c) => setSelectedId(c.user_id)}
            scope={scope}
          />
        )}
      </motion.div>
    )
  })()

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Danh bạ">
        <header className="flex items-center gap-4 px-5 pt-4 pb-3 min-w-0">
          <div className="grid gap-0.5 min-w-0">
            <Heading as="h1" look="page" className="truncate">Danh bạ</Heading>
            <span className="text-sm text-ink-muted truncate">
              {loading ? 'Đang tải…' : `${all.length} người trong ${workspace}`}
            </span>
          </div>
          <div role="group" aria-label="Kiểu xem" className="ml-auto hidden sm:inline-flex gap-1 p-0.75 rounded-lg bg-sunk shrink-0">
            {([['table', 'Bảng', List], ['cards', 'Thẻ', LayoutGrid]] as const).map(([id, label, Icon]) => (
              <Pressable
                key={id}
                aria-pressed={prefs.contactsView === id}
                aria-label={label}
                title={label}
                onClick={() => update({ contactsView: id })}
                className={`grid place-items-center w-9 h-8 rounded-md transition-colors duration-quick
                  ${prefs.contactsView === id ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'}`}
              >
                <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
              </Pressable>
            ))}
          </div>
        </header>

        <div className="flex flex-wrap items-center gap-2 px-5 pb-3">
          <SearchField
            label="Tên, chức danh hoặc email"
            moduleSearch
            tone="sunk"
            value={filter.query}
            onChange={(e) => setFilter((f) => ({ ...f, query: e.target.value }))}
            className="w-full sm:w-72"
          />
          <DepartmentFilter
            departments={departments}
            value={filter.department}
            onChange={(department) => setFilter((f) => ({ ...f, department }))}
          />
          <FilterChip pressed={filter.onlineOnly} onClick={() => setFilter((f) => ({ ...f, onlineOnly: !f.onlineOnly }))}>
            Đang trực tuyến
          </FilterChip>
        </div>

        {connectionLost && (
          <div role="status" className="mx-5 mb-2 px-3 py-2 rounded-surface bg-warning-wash text-sm">
            Đang kết nối lại… Trạng thái trực tuyến có thể chưa mới nhất.
          </div>
        )}

        <div className="flex-1 min-h-0 overflow-y-auto">{body}</div>
      </section>

      <AnimatePresence>
        {selected && (
          <ContactPanel
            key="profile"
            contact={selected}
            online={isOnline(selected)}
            colleagues={colleaguesOf(all, selected)}
            isMe={selected.user_id === me?.id}
            messaging={createDM.isPending}
            onMessage={message}
            onClose={() => setSelectedId(null)}
          />
        )}
      </AnimatePresence>
    </div>
  )
}
