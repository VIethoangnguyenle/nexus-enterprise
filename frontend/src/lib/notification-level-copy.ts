import type { NotificationLevel } from '../api/notification-levels'
import type { ConversationKind } from './conversations'

/** One choice of the notification menu and sheet (design/mockups/notification-levels.html §2, §5). */
export interface LevelOption {
  level: NotificationLevel
  title: string
  /** The line under the title: what the level does. */
  description: string
  /** The default of this kind of conversation; it carries the "Mặc định" tag. */
  isDefault?: boolean
}

const SPACE: LevelOption[] = [
  { level: 'all', title: 'Tất cả', description: 'Mọi tin và mọi trả lời trong thread đều tính vào số chưa đọc và báo cho bạn.' },
  {
    level: 'main', title: 'Tin chính và thread đang theo dõi', isDefault: true,
    description: 'Tin mới trong luồng và trả lời trong thread bạn theo dõi. Trả lời ở thread khác chỉ cập nhật số trả lời.',
  },
  { level: 'off', title: 'Tắt', description: 'Không tính số chưa đọc, không thông báo. Vẫn báo khi có người nhắc tên bạn.' },
]

/** A direct message has no middle level: every message is for the reader, so it is on or off. */
const DM: LevelOption[] = [
  { level: 'main', title: 'Bật', isDefault: true, description: 'Mọi tin nhắn đều tính vào số chưa đọc và báo cho bạn.' },
  { level: 'off', title: 'Tắt', description: 'Không tính số chưa đọc, không thông báo.' },
]

/** The choices a conversation of this kind offers, in menu order. A direct message is never offered "all". */
export function levelOptions(kind: ConversationKind): LevelOption[] {
  return kind === 'dm' ? DM : SPACE
}

/** The option that stands for `level` here; a stale "all" on a direct message reads as the default. */
export function optionFor(kind: ConversationKind, level: NotificationLevel): LevelOption {
  const options = levelOptions(kind)
  return options.find((o) => o.level === level) ?? options.find((o) => o.isDefault) ?? options[0]!
}
