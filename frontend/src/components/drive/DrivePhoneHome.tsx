import type { ReactNode } from 'react'
import { CircleAlert, Clock, Star } from 'lucide-react'
import type { DriveItem, QuickAccessEntry } from '../../api/drive'
import { groupFiles } from '../../lib/file-groups'
import { formatListTime, toMillis } from '../../lib/format'
import { Button, Pressable } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { FileThumb } from '../files/FileThumb'
import { itemVersion } from './drive-model'

/** Where the phone's Tài liệu home is: the chips under its title. Bản nháp leaves for Văn bản. */
export type PhoneTab = 'recent' | 'folders' | 'starred' | 'shared'

const CHIPS: { id: PhoneTab | 'drafts'; label: string }[] = [
  { id: 'recent', label: 'Gần đây' },
  { id: 'folders', label: 'Thư mục' },
  { id: 'starred', label: 'Gắn sao' },
  { id: 'shared', label: 'Chia sẻ với tôi' },
  { id: 'drafts', label: 'Bản nháp' },
]

/**
 * The entry chips that replace the folder tree on a phone (DESIGN.md "Tài
 * liệu", mockup §10): Gần đây · Thư mục · Gắn sao · Chia sẻ với tôi · Bản nháp.
 */
export function DrivePhoneChips({ tab, onTab, onDrafts }: {
  tab: PhoneTab
  onTab: (tab: PhoneTab) => void
  onDrafts: () => void
}) {
  return (
    <nav aria-label="Lối vào Tài liệu" className="flex gap-2 px-4 pb-2 overflow-x-auto [scrollbar-width:none]">
      {CHIPS.map((c) => {
        const on = c.id === tab
        return (
          <Pressable
            key={c.id}
            aria-current={on ? 'page' : undefined}
            onClick={() => (c.id === 'drafts' ? onDrafts() : onTab(c.id))}
            className={`press shrink-0 h-9 px-3 rounded-lg whitespace-nowrap text-small-ui transition-colors duration-quick
              ${on ? 'bg-accent-wash font-semibold text-ink' : 'bg-sunk font-medium text-ink hover:bg-hover'}`}
          >
            {c.label}
          </Pressable>
        )
      })}
    </nav>
  )
}

/** The line under a Truy cập nhanh card, from what the server says the reason is. Never an id. */
export function reasonLine(e: QuickAccessEntry): string {
  if (e.reason === 'shared') return e.shared_by_name ? `${e.shared_by_name} chia sẻ` : 'Được chia sẻ với bạn'
  if (e.reason === 'starred') return 'Bạn gắn sao'
  return ['Của bạn', formatListTime(e.at)].filter(Boolean).join(' · ')
}

const SECTION = 'px-4 pt-3 pb-1.5 text-xs font-semibold uppercase tracking-wide text-ink-muted'

/** Truy cập nhanh: up to six cards of 148 that scroll sideways. */
export function DriveQuickAccess({ entries, onOpen }: { entries: QuickAccessEntry[]; onOpen: (item: DriveItem) => void }) {
  if (entries.length === 0) return null
  return (
    <section aria-labelledby="drive-quick-access">
      <h2 id="drive-quick-access" className={SECTION}>Truy cập nhanh</h2>
      <ul className="flex gap-2.5 px-4 pb-2 overflow-x-auto [scrollbar-width:none]">
        {entries.map((e) => (
          <li key={e.item.id} className="shrink-0 w-37">
            <Pressable
              onClick={() => onOpen(e.item)}
              aria-label={`${e.item.name}, ${reasonLine(e)}`}
              className="press grid gap-1.5 w-full p-1.5 pb-2 rounded-xl bg-raised"
            >
              <FileThumb
                fileId={e.item.id}
                name={e.item.name}
                mimeType={e.item.mime_type}
                version={itemVersion(e.item)}
                aspect="16 / 10"
                radius={9}
                iconSize={24}
              />
              <span className="grid min-w-0 px-0.5">
                <b className="text-[0.8125rem] font-semibold truncate">{e.item.name}</b>
                <small className="text-xs text-ink-muted truncate">{reasonLine(e)}</small>
              </span>
            </Pressable>
          </li>
        ))}
      </ul>
    </section>
  )
}

/**
 * Gần đây below Truy cập nhanh: the workspace's files the person can read,
 * grouped Hôm nay · Tuần này · Tháng này · Cũ hơn.
 */
export function DriveRecentGroups({ items, renderList }: {
  items: DriveItem[]
  renderList: (items: DriveItem[], label: string) => ReactNode
}) {
  return (
    <>
      {groupFiles(items, (i) => toMillis(i.updated_at)).map((g) => (
        <section key={g.key} aria-labelledby={`drive-recent-${g.key}`}>
          <h2 id={`drive-recent-${g.key}`} className={SECTION}>{g.label}</h2>
          {renderList(g.items, `Sửa ${g.label.toLowerCase()}`)}
        </section>
      ))}
    </>
  )
}

/** What fills the phone's home under Gần đây and Gắn sao: rows, or why there are none. */
export function PhoneHomeBody({ tab, loading, isError, empty, filtered, onRetry, content }: {
  tab: PhoneTab
  loading: boolean
  isError: boolean
  empty: boolean
  /** A search narrows the list. */
  filtered: boolean
  onRetry: () => void
  content: ReactNode
}) {
  if (isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được danh sách tệp. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" size="sm" onClick={onRetry}>Thử lại</Button>}
      />
    )
  }
  if (loading) {
    return (
      <div aria-busy="true" className="grid gap-1 px-4 pt-3">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="flex items-center gap-3 h-15">
            <span className="skeleton size-10 rounded-surface" />
            <span className="skeleton h-3.5 rounded-sm" style={{ width: `${70 - i * 8}%` }} />
          </div>
        ))}
      </div>
    )
  }
  if (empty) {
    return tab === 'starred' ? (
      <EmptyState
        icon={<Star size={24} strokeWidth={1.75} />}
        text={filtered ? 'Không có mục gắn sao nào khớp.' : 'Chưa có mục nào được gắn sao. Vuốt một hàng sang trái hoặc mở ⋯ để gắn sao.'}
      />
    ) : (
      <EmptyState
        icon={<Clock size={24} strokeWidth={1.75} />}
        text={filtered ? 'Không có tệp nào khớp.' : 'Chưa có tệp nào gần đây. Tệp bạn và mọi người tải lên sẽ hiện ở đây.'}
      />
    )
  }
  return <>{content}</>
}
