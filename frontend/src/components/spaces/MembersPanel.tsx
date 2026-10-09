import { useMemo, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { MoreVertical, MessageSquare, UserMinus, UserPlus, CircleAlert, Users } from 'lucide-react'
import { useAddChannelMember, useChannelMembers, useCreateDM, useRemoveChannelMember } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useMotionPresets } from '../../lib/motion'
import { isForbidden, explain } from '../../lib/errors'
import { displayName, matchesPerson, type PeopleDirectory, type Person } from '../../lib/people'
import type { ChannelMember } from '../../api/messaging'
import {
  Avatar, Button, IconButton, MenuItem, MenuSeparator, PeoplePicker, Popover, SearchField, toast,
} from '../primitives'
import { SidePanel } from './SidePanel'
import { EmptyState } from './EmptyState'

interface MembersPanelProps {
  channelId: string
  /** DMs list their two people but cannot be changed. */
  kind: 'dm' | 'space'
  spaceName: string
  people: PeopleDirectory
  me: string | undefined
  /** Open with the people picker showing ("Thêm người" in the header). */
  startAdding?: boolean
  onClose: () => void
}

interface Row {
  member: ChannelMember
  person: Person
  isMe: boolean
}

/**
 * Members of a space (design/mockups/spaces.html §4): search, add through the
 * people picker, and per person "Nhắn tin trực tiếp" or "Xoá khỏi nhóm".
 * Removing does not ask first; the toast offers "Hoàn tác" for 5s instead.
 * Manager roles are not shown: the API does not return a member's role.
 */
export function MembersPanel({ channelId, kind, spaceName, people, me, startAdding, onClose }: MembersPanelProps) {
  const m = useMotionPresets()
  const { data, isLoading, isError, refetch } = useChannelMembers(channelId)
  const add = useAddChannelMember(channelId)
  const remove = useRemoveChannelMember(channelId)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const [query, setQuery] = useState('')
  const [adding, setAdding] = useState(!!startAdding && kind === 'space')
  const [picked, setPicked] = useState<Person[]>([])
  const [saving, setSaving] = useState(false)
  const editable = kind === 'space'

  const rows: Row[] = useMemo(() => {
    const list = (data?.members ?? []).map((mem) => {
      const known = people.byUserId.get(mem.user_id) ?? people.byNodeId.get(mem.ngac_node_id)
      const person: Person = known ?? {
        userId: mem.user_id,
        nodeId: mem.ngac_node_id,
        username: mem.username,
        name: displayName(people, mem.user_id, mem.username),
        role: '',
        avatarUrl: '',
      }
      return { member: mem, person, isMe: !!me && mem.user_id === me }
    })
    return list.sort((a, b) => (a.isMe === b.isMe ? a.person.name.localeCompare(b.person.name, 'vi') : a.isMe ? -1 : 1))
  }, [data, people, me])

  const visible = rows.filter((r) => matchesPerson(r.person, query))
  const memberIds = useMemo(() => new Set(rows.map((r) => r.person.userId).filter(Boolean)), [rows])

  const addPeople = async (list: Person[]) => {
    setSaving(true)
    const results = await Promise.allSettled(
      list.map((p) => add.mutateAsync({ ngacNodeId: p.nodeId, username: p.username, userId: p.userId })),
    )
    setSaving(false)
    const failed = results.filter((r): r is PromiseRejectedResult => r.status === 'rejected')
    const ok = list.length - failed.length
    if (ok > 0) toast(ok === 1 ? `Đã thêm ${list[0]!.name} vào nhóm` : `Đã thêm ${ok} người vào nhóm`)
    if (failed.length > 0) {
      toast.error(
        isForbidden(failed[0]!.reason)
          ? 'Bạn chưa có quyền thêm người vào nhóm này. Nhờ quản lý nhóm thêm giúp.'
          : explain(failed[0]!.reason, 'thêm người vào nhóm'),
      )
    }
    return failed.length === 0
  }

  const removeMember = (row: Row) => {
    remove.mutate(
      { nodeId: row.member.ngac_node_id },
      {
        onSuccess: () =>
          toast(`Đã xoá ${row.person.name} khỏi nhóm`, {
            action: { label: 'Hoàn tác', onClick: () => void addPeople([row.person]) },
          }),
        onError: (err) =>
          toast.error(
            isForbidden(err)
              ? `Bạn chưa có quyền xoá người khỏi nhóm này. Nhờ quản lý nhóm làm giúp.`
              : explain(err, `xoá ${row.person.name} khỏi nhóm`),
          ),
      },
    )
  }

  return (
    <SidePanel
      label="Thành viên"
      title="Thành viên"
      sub={<span className="tnum">{rows.length} người</span>}
      closeLabel="danh sách thành viên"
      onClose={onClose}
      actions={
        editable && !adding ? (
          <Button variant="primary" size="sm" onClick={() => setAdding(true)}>
            <UserPlus size={16} strokeWidth={1.75} aria-hidden="true" /> Thêm
          </Button>
        ) : undefined
      }
    >
      <AnimatePresence initial={false}>
        {adding && (
          <motion.div key="add" {...m.row} className="overflow-hidden">
            <form
              className="grid gap-2 px-3 pb-3"
              onSubmit={(e) => {
                e.preventDefault()
                if (!picked.length) return
                void addPeople(picked).then((done) => {
                  if (done) {
                    setPicked([])
                    setAdding(false)
                  }
                })
              }}
            >
              <PeoplePicker
                label={`Thêm người vào ${spaceName}`}
                people={people.list}
                value={picked}
                onChange={setPicked}
                exclude={memberIds}
                autoFocus
              />
              <div className="flex justify-end gap-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => { setAdding(false); setPicked([]) }}>
                  Huỷ
                </Button>
                <Button type="submit" variant="primary" size="sm" disabled={!picked.length} loading={saving}>
                  Thêm vào nhóm
                </Button>
              </div>
            </form>
          </motion.div>
        )}
      </AnimatePresence>

      <SearchField
        label="Tìm thành viên"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        className="mx-3 mb-2"
      />

      {isError ? (
        <EmptyState
          compact
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Chưa tải được danh sách thành viên."
          action={<Button variant="soft" size="sm" onClick={() => refetch()}>Thử lại</Button>}
        />
      ) : isLoading ? (
        <div className="grid gap-1 px-3" aria-busy="true" aria-label="Đang tải">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-2.5 py-2">
              <div className="skeleton w-8 h-8 rounded-full" />
              <div className="skeleton h-3.5 w-1/2 rounded-sm" />
            </div>
          ))}
        </div>
      ) : visible.length === 0 ? (
        <EmptyState
          compact
          icon={<Users size={24} strokeWidth={1.75} />}
          text={query ? `Không có thành viên nào khớp “${query}”.` : 'Nhóm chưa có thành viên.'}
          action={query ? <Button variant="soft" size="sm" onClick={() => setQuery('')}>Xoá tìm kiếm</Button> : undefined}
        />
      ) : (
        <>
          <div className="text-label text-ink-muted px-3 pt-1.5 pb-1.5">Thành viên</div>
          <ul className="grid gap-0.5 list-none m-0 p-0">
            <AnimatePresence initial={false}>
              {visible.map((r) => (
                <motion.li key={r.member.ngac_node_id || r.member.user_id} {...m.row} className="overflow-hidden">
                  <MemberRow
                    row={r}
                    online={!!onlineUsers[r.person.userId]}
                    canRemove={editable && !r.isMe}
                    onRemove={() => removeMember(r)}
                  />
                </motion.li>
              ))}
            </AnimatePresence>
          </ul>
        </>
      )}
    </SidePanel>
  )
}

function MemberRow({ row, online, canRemove, onRemove }: {
  row: Row
  online: boolean
  canRemove: boolean
  onRemove: () => void
}) {
  const navigate = useNavigate()
  const createDM = useCreateDM()
  const moreRef = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const { person, isMe } = row

  const message = () =>
    createDM.mutate(
      { userId: person.userId, ngacNodeId: person.nodeId },
      {
        onSuccess: (ch) => navigate({ to: '/channels/$channelId', params: { channelId: ch.id } }),
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${person.name}`)),
      },
    )

  return (
    <div className="grid grid-cols-[32px_minmax(0,1fr)_auto] gap-2.5 items-center px-3 py-2 rounded-surface hover:bg-hover">
      <Avatar name={person.name} hueKey={person.userId} src={person.avatarUrl} online={online} size={32} />
      <span className="grid min-w-0">
        <span className="font-medium text-ink truncate">{isMe ? `${person.name} (bạn)` : person.name}</span>
        {person.role && <span className="text-xs text-ink-muted truncate">{person.role}</span>}
      </span>
      {!isMe && (
        <>
          <IconButton
            ref={moreRef}
            size="sm"
            aria-label={`Tuỳ chọn cho ${person.name}`}
            aria-haspopup="menu"
            aria-expanded={open}
            onClick={() => setOpen((o) => !o)}
          >
            <MoreVertical size={16} strokeWidth={1.75} />
          </IconButton>
          <Popover open={open} onClose={() => setOpen(false)} anchorRef={moreRef} placement="bottom-end" role="menu" label={`Tuỳ chọn cho ${person.name}`}>
            <MenuItem icon={<MessageSquare size={16} strokeWidth={1.75} />} onClick={message}>
              Nhắn tin trực tiếp
            </MenuItem>
            {canRemove && (
              <>
                <MenuSeparator />
                <MenuItem tone="danger" icon={<UserMinus size={16} strokeWidth={1.75} />} onClick={onRemove}>
                  Xoá khỏi nhóm
                </MenuItem>
              </>
            )}
          </Popover>
        </>
      )}
    </div>
  )
}
