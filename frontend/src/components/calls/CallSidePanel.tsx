import { useState } from 'react'
import { Mic, MicOff, Video, VideoOff, X } from 'lucide-react'
import type { Call } from '../../api/calls'
import type { CallSnapshot, ParticipantView } from '../../lib/call-room'
import { formatTime } from '../../lib/format'
import { displayName } from '../../lib/people'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useInviteToCall } from '../../hooks/useCalls'
import { useChannelMembers } from '../../hooks/useMessaging'
import { usePeople } from '../../hooks/usePeople'
import { useWebSocketStore } from '../../stores/websocket.store'
import type { CallPanel } from '../../stores/call.store'
import { Avatar, Button, IconButton, Pressable, toast } from '../primitives'
import { CallChat } from './CallChat'
import './call-stage.css'

interface Props {
  panel: CallPanel
  onPanel: (p: CallPanel | null) => void
  channelId: string
  callId: string
  title: string
  isDm: boolean
  call: Call | undefined
  snapshot: CallSnapshot
}

/** The right panel of the stage (340 wide): Chat, and Người tham gia with raised hands, who is in, and who to invite. */
export function CallSidePanel({ panel, onPanel, channelId, callId, title, isDm, call, snapshot }: Props) {
  const inCall = snapshot.participants.length
  // The call of an event is in no conversation: the panel is the list of people and nothing else.
  const hasChat = !!channelId
  const shown = hasChat ? panel : 'people'
  return (
    <aside className="cs-panel" aria-label="Bảng bên">
      <div className="cs-panel-head">
        <b>{shown === 'chat' ? 'Chat trong cuộc gọi' : 'Người tham gia'}</b>
        <IconButton aria-label="Đóng bảng" onClick={() => onPanel(null)}><X size={18} /></IconButton>
      </div>
      {hasChat && (
        <div className="cs-tabs" role="tablist">
          <Pressable role="tab" className="cs-tab" aria-selected={panel === 'chat'} onClick={() => onPanel('chat')}>Chat</Pressable>
          <Pressable role="tab" className="cs-tab" aria-selected={panel === 'people'} onClick={() => onPanel('people')}>
            {`Người tham gia · ${inCall}`}
          </Pressable>
        </div>
      )}
      <div className="cs-panel-body">
        {shown === 'chat'
          ? <CallChat channelId={channelId} title={title} />
          : <PeopleTab channelId={channelId} callId={callId} isDm={isDm} call={call} snapshot={snapshot} />}
      </div>
    </aside>
  )
}

function subtitleOf(p: ParticipantView, startedBy: string | undefined, role: string): string {
  return [
    startedBy && p.id === startedBy ? 'Người bắt đầu' : '',
    p.speaking ? 'đang nói' : '',
    p.quality === 'poor' ? 'Mạng yếu' : '',
    role,
  ].filter(Boolean).join(' · ')
}

function PeopleTab({ channelId, callId, isDm, call, snapshot }: Pick<Props, 'channelId' | 'callId' | 'isDm' | 'call' | 'snapshot'>) {
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const members = useChannelMembers(channelId)
  const online = useWebSocketStore((s) => s.onlineUsers)
  const invite = useInviteToCall(callId)
  const [invited, setInvited] = useState<Set<string>>(new Set())

  const hands = snapshot.participants.filter((p) => p.handRank !== null).sort((a, b) => (a.handRank ?? 0) - (b.handRank ?? 0))
  const inRoom = new Set(snapshot.participants.map((p) => p.id))
  const waiting = isDm ? [] : (members.data?.members ?? []).filter((m) => !inRoom.has(m.user_id))

  const ask = (userId: string) => {
    invite.mutate([userId], {
      onSuccess: () => {
        setInvited((s) => new Set(s).add(userId))
        toast('Đã gửi lời mời vào cuộc gọi.')
      },
    })
  }

  return (
    <div className="cs-people">
      {hands.length > 0 && (
        <>
          <div className="cs-plabel">Đang giơ tay</div>
          {hands.map((p) => (
            <div key={`h-${p.id}`} className="cs-prow">
              <Avatar name={p.name} hueKey={p.id} size={32} src={people.byUserId.get(p.id)?.avatarUrl} />
              <span><b>{p.isLocal ? 'Bạn' : p.name}</b><small>{p.handAt ? `Giơ tay lúc ${formatTime(p.handAt / 1000)}` : 'Đang giơ tay'}</small></span>
              <span className="cs-handq" aria-label={`Thứ ${p.handRank}`}>{p.handRank}</span>
            </div>
          ))}
        </>
      )}
      <div className="cs-plabel">{`Trong cuộc gọi · ${snapshot.participants.length}`}</div>
      {snapshot.participants.map((p) => {
        const person = people.byUserId.get(p.id)
        return (
          <div key={p.id} className="cs-prow">
            <Avatar name={p.name} hueKey={p.id} size={32} src={person?.avatarUrl} />
            <span>
              <b>{p.isLocal ? 'Bạn' : p.name}</b>
              <small>{subtitleOf(p, call?.started_by, person?.role ?? '')}</small>
            </span>
            <span className="cs-pstate">
              {p.micOn ? <Mic className="cs-icon-s" aria-label="Micro bật" /> : <MicOff className="cs-icon-s cs-off" aria-label="Micro tắt" />}
              {p.camOn ? <Video className="cs-icon-s" aria-label="Camera bật" /> : <VideoOff className="cs-icon-s cs-off" aria-label="Camera tắt" />}
            </span>
          </div>
        )
      })}
      {waiting.length > 0 && (
        <>
          <div className="cs-plabel">{`Trong nhóm, chưa vào · ${waiting.length}`}</div>
          {waiting.map((m) => {
            const name = displayName(people, m.user_id, m.username)
            const sent = invited.has(m.user_id)
            return (
              <div key={m.user_id} className="cs-prow">
                <Avatar name={name} hueKey={m.user_id} size={32} src={people.byUserId.get(m.user_id)?.avatarUrl} />
                <span><b>{name}</b><small>{online[m.user_id] ? 'Đang hoạt động' : ''}</small></span>
                <Button variant="soft" size="sm" disabled={sent || invite.isPending} onClick={() => ask(m.user_id)}>
                  {sent ? 'Đã mời' : 'Mời'}
                </Button>
              </div>
            )
          })}
        </>
      )}
    </div>
  )
}
