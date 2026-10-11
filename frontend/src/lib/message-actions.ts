import type { Message } from '../api/messaging'
import { cardKindOf, isSystemLine } from './message-model'

type Pending = Message & { _optimistic?: boolean }

/** Stored, live and said by a person (or a card): the only messages that can be starred or quoted. */
const isRealMessage = (m: Pending): boolean =>
  !m._optimistic && !!m.id && !m.id.startsWith('temp-') && !m.deleted && (!isSystemLine(m) || !!cardKindOf(m))

/** "Gắn sao tin": not offered on a message still being sent, a deleted one, or a line the product wrote. */
export const canStar = isRealMessage

/** "Trích dẫn khi trả lời": the server refuses a quote of a message that is not live in the same channel. */
export const canQuote = isRealMessage
