import { useState, type ComponentProps } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { messagingApi } from '../../api/messaging'
import type { DriveItem } from '../../api/drive'
import { calendarsQueryOptions } from '../../hooks/useCalendar'
import { useSubscribe } from '../../hooks/useCalendarSharing'
import { commonWorkKeys, useCommonWork } from '../../hooks/useCommonWork'
import { useMyGroups } from '../../hooks/useContactCategories'
import { keys } from '../../hooks/keys'
import { explain } from '../../lib/errors'
import { queryClient } from '../../lib/query-client'
import { Avatar, toast } from '../primitives'
import { NavSheet, SheetItem } from '../spaces/NavSheet'
import { CommonWork } from './CommonWork'
import { ContactProfile } from './ContactProfile'
import { nameOf } from './contacts-model'

type BaseProps = Omit<ComponentProps<typeof ContactProfile>, 'mode' | 'onCalendar' | 'onAddToGroup' | 'onInviteToMeeting' | 'children' | 'onEdit'>

interface ContactPhoneProfileProps extends BaseProps {
  wsId: string
  onOpenChat: (channelId: string) => void
}

/**
 * The profile card as a phone shows it (DESIGN.md "Danh bạ" › Điện thoại, mockup `mobile-shell.html` §12), with what
 * only the phone offers: Lịch (the person's calendar joins Lịch through the existing "Tìm người" path, as far as their
 * sharing lets the viewer see it), ⋯ › Thêm vào nhóm… (one of the viewer's own groups) and Mời vào cuộc họp, "Đã cùng
 * làm việc", and on one's own card "Sửa hồ sơ" opening Tôi › Tài khoản và hồ sơ.
 */
export function ContactPhoneProfile({ wsId, onOpenChat, ...profile }: ContactPhoneProfileProps) {
  const navigate = useNavigate()
  const c = profile.contact
  const name = nameOf(c)
  const [picking, setPicking] = useState(false)
  const subscribe = useSubscribe(wsId)
  const groups = useMyGroups(wsId, picking)
  const person = profile.isMe ? null : { userId: c.user_id, nodeId: c.ngac_node_id }
  const common = useCommonWork(wsId, person)
  const shared = new Set(common.groups.items.map((g) => g.id))

  const toCalendar = () => void navigate({ to: '/calendar', search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })
  const openCalendar = async () => {
    // The viewer's calendars, asked for only now: one already listed is opened, not added again. When the list cannot
    // be had, adding is tried and the calendar service decides what the viewer may see.
    const list = await queryClient.fetchQuery(calendarsQueryOptions(wsId)).catch(() => undefined)
    if (list?.calendars?.some((cal) => cal.owner?.nodeId === c.ngac_node_id)) toCalendar()
    else subscribe.mutate({ nodeId: c.ngac_node_id, name }, { onSuccess: toCalendar })
  }
  const add = useMutation({
    meta: { silentError: true },
    mutationFn: (channelId: string) => messagingApi.addMember(channelId, c.ngac_node_id),
    onSuccess: (_r, channelId) => {
      const group = groups.groups.find((g) => g.id === channelId)
      toast(group ? `Đã thêm ${name} vào ${group.name}` : `Đã thêm ${name} vào nhóm`)
    },
    onError: (err) => toast.error(explain(err, `thêm ${name} vào nhóm`)),
    onSettled: (_r, _e, channelId) => {
      void queryClient.invalidateQueries({ queryKey: keys.messaging.members(channelId) })
      void queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
      void queryClient.invalidateQueries({ queryKey: commonWorkKeys.groups(wsId, c.ngac_node_id) })
    },
  })
  const openFile = (item: DriveItem) => {
    // A folder opens itself; a file is listed among the files shared with the viewer.
    if (item.item_type === 'folder') void navigate({ to: '/drive', search: (prev: { ws?: string }) => ({ ...(prev.ws ? { ws: prev.ws } : {}), folder: item.id }) })
    else void navigate({ to: '/drive', search: (prev: { ws?: string }) => ({ ...(prev.ws ? { ws: prev.ws } : {}), view: 'shared' as const }) })
  }
  const choices = groups.groups.filter((g) => !shared.has(g.id))

  return (
    <>
      <ContactProfile
        {...profile}
        mode="screen"
        onEdit={() => void navigate({ to: '/settings/$section', params: { section: 'account' }, search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })}
        onCalendar={() => void openCalendar()}
        onAddToGroup={() => setPicking(true)}
        onInviteToMeeting={() => void navigate({
          to: '/calendar/new', search: (prev: { ws?: string }) => ({ ...(prev.ws ? { ws: prev.ws } : {}), title: `Họp với ${name}` }),
        })}
      >
        {person && <CommonWork wsId={wsId} person={person} onOpenGroup={onOpenChat} onOpenFile={openFile} />}
      </ContactProfile>
      {picking && (
        <NavSheet label={`Thêm ${name} vào nhóm`} onClose={() => setPicking(false)}>
          {groups.isLoading && <p className="m-0 px-3.5 py-3 text-sm text-ink-muted">Đang tải nhóm của bạn…</p>}
          {groups.isError && <p className="m-0 px-3.5 py-3 text-sm text-ink-muted">Không tải được nhóm của bạn.</p>}
          {groups.known && choices.length === 0 && (
            <p className="m-0 px-3.5 py-3 text-sm text-ink-muted">Không còn nhóm nào của bạn để thêm {name} vào.</p>
          )}
          {choices.map((g) => (
            <SheetItem
              key={g.id}
              icon={<Avatar name={g.name} hueKey={g.id} src={g.avatar_url || undefined} shape="square" size={24} />}
              disabled={add.isPending}
              onClick={() => { setPicking(false); add.mutate(g.id) }}
            >
              {g.name}
            </SheetItem>
          ))}
        </NavSheet>
      )}
    </>
  )
}
