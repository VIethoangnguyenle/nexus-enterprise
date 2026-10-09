import type { Conversation } from '../../lib/conversations'
import { Avatar, SpaceIcon } from '../primitives'

/** A DM shows the other person's avatar (round); a space its icon (rounded square). */
export function ConversationIcon({ conversation: c, size, online, halo }: {
  conversation: Pick<Conversation, 'kind' | 'title' | 'hueKey' | 'partner'>
  size: 24 | 32 | 40
  online?: boolean
  halo?: boolean
}) {
  if (c.kind === 'dm') {
    return <Avatar name={c.title} hueKey={c.hueKey} src={c.partner?.avatarUrl} size={size} online={online} halo={halo} />
  }
  return <SpaceIcon name={c.title} hueKey={c.hueKey} size={size} />
}
