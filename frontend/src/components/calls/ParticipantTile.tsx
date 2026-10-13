import { Hand, Mic, MicOff, Pin, Signal } from 'lucide-react'
import { Avatar, Pressable } from '../primitives'
import type { ParticipantView } from '../../lib/call-room'
import { TrackVideo } from './media'
import './call-stage.css'

interface Props {
  person: ParticipantView
  /** The picture to show: the camera, or the shared screen when this tile is the presentation. */
  show?: 'camera' | 'screen'
  /** "Bạn (Lê Thị Hoa)" in the grid, plain "Bạn" in a strip or corner. */
  longSelf?: boolean
  avatarSize?: 40 | 52 | 64
  avatarUrl?: string
  pinned?: boolean
  onPin?: (id: string) => void
  /** Marks the video Picture-in-Picture may use. */
  speakerMark?: boolean
  /** Phone tiles are small: the pin and the network chip stay out. */
  compact?: boolean
}

/** One person: video or avatar, name, microphone, network, raised hand, and a ring while speaking. */
export function ParticipantTile({
  person, show = 'camera', longSelf = false, avatarSize = 64, avatarUrl, pinned, onPin, speakerMark, compact,
}: Props) {
  const track = show === 'screen' ? person.screen : person.camera
  const label = person.isLocal ? (longSelf ? `Bạn (${person.name})` : 'Bạn') : person.name
  const showVideo = !!track
  return (
    <div
      className={`cs-tile ${person.speaking ? 'cs-tile-speaking' : ''}`}
      data-testid={`tile-${person.id}`}
      aria-label={label}
    >
      {showVideo ? (
        <TrackVideo
          track={track}
          mirror={person.isLocal && show === 'camera'}
          fit={show === 'screen' ? 'contain' : 'cover'}
          speakerMark={speakerMark}
        />
      ) : (
        <span className="cs-avatar"><Avatar name={person.name} hueKey={person.id} size={avatarSize} src={avatarUrl} /></span>
      )}
      {person.handRank !== null && show === 'camera' && (
        <span className="cs-hand" aria-label={`Đang giơ tay, thứ ${person.handRank}`}>
          <Hand className="cs-icon-xs" aria-hidden="true" />{person.handRank}
        </span>
      )}
      {!compact && person.quality === 'poor' && (
        <span className="cs-poor"><Signal className="cs-icon-xs" aria-hidden="true" />Mạng yếu</span>
      )}
      {show === 'camera' && (
        <span className={`cs-mic ${person.micOn ? '' : 'cs-mic-off'}`} role="img" aria-label={person.micOn ? 'Micro bật' : 'Micro tắt'}>
          {!person.micOn ? <MicOff className="cs-icon-s" /> : person.speaking ? (
            <span className="cs-wave" aria-hidden="true"><i /><i /><i /></span>
          ) : <Mic className="cs-icon-s" />}
        </span>
      )}
      {show === 'screen' ? (
        <span className="cs-by">{person.isLocal ? 'Bạn đang trình bày' : `${person.name} đang trình bày`}</span>
      ) : (
        <span className="cs-name">{label}</span>
      )}
      {onPin && !compact && show === 'camera' && (
        <Pressable
          className="cs-pin"
          aria-pressed={!!pinned}
          aria-label={pinned ? `Bỏ ghim ${label}` : `Ghim ${label}`}
          onClick={() => onPin(person.id)}
        >
          <Pin className="cs-icon-s" />
        </Pressable>
      )}
    </div>
  )
}
