import { useState, type ReactNode } from 'react'
import { ChevronRight, FileText, Folder, MessageSquare, RefreshCw } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import { useCommonWork, type CommonWorkPerson, type CommonWorkSource } from '../../hooks/useCommonWork'
import { Avatar, Pressable } from '../primitives'
import { NavSheet, SheetItem } from '../spaces/NavSheet'

interface CommonWorkProps {
  wsId: string
  person: CommonWorkPerson
  /** A group of the list: its conversation opens. */
  onOpenGroup: (channelId: string) => void
  /** A file or folder of the list: where it lives in Tài liệu opens. */
  onOpenFile: (item: DriveItem) => void
}

const ROW = `press flex items-center gap-3 w-full min-w-0 min-h-14 px-3 py-2 border-none bg-transparent text-left cursor-pointer
  rounded-lg hover:bg-hover focus-ring transition-colors duration-quick`

function Row({ icon, title, sub, onPress }: { icon: ReactNode; title: string; sub: string; onPress: () => void }) {
  return (
    <Pressable onClick={onPress} aria-haspopup="dialog" className={ROW}>
      <span aria-hidden="true" className="grid place-items-center shrink-0 w-10 h-10 rounded-lg bg-accent-wash text-accent">{icon}</span>
      <span className="grid flex-1 min-w-0">
        <span className="truncate font-medium text-ink">{title}</span>
        <span className="truncate text-small text-ink-muted">{sub}</span>
      </span>
      <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />
    </Pressable>
  )
}

function Failed({ text, onRetry }: { text: string; onRetry: () => void }) {
  return (
    <div className="flex items-center gap-2 min-h-12 px-3 text-small text-ink-muted">
      <span className="flex-1 min-w-0">{text}</span>
      <Pressable onClick={onRetry} className="inline-flex items-center gap-1 h-9 px-2 rounded-lg font-medium text-accent hover:bg-hover focus-ring">
        <RefreshCw size={14} strokeWidth={1.75} aria-hidden="true" />
        Thử lại
      </Pressable>
    </div>
  )
}

function Skeleton() {
  return (
    <div aria-hidden="true" className="flex items-center gap-3 min-h-14 px-3">
      <span className="skeleton w-10 h-10 rounded-lg" />
      <span className="grid gap-1.5 flex-1">
        <span className="skeleton h-3.5 w-1/2 rounded-sm" />
        <span className="skeleton h-3 w-1/3 rounded-sm" />
      </span>
    </div>
  )
}

const settled = <T,>(s: CommonWorkSource<T>) => !s.loading && !s.failed

/**
 * "Đã cùng làm việc" on a phone's profile card (DESIGN.md "Danh bạ" › Điện thoại, mockup §12): "N nhóm chung" with
 * their names, and "Tệp đã chia sẻ với bạn" with how many, each opening its list. Only what the two services let the
 * signed-in person read; a source with nothing is left out, and the block too when both have nothing.
 */
export function CommonWork({ wsId, person, onOpenGroup, onOpenFile }: CommonWorkProps) {
  const { groups, files } = useCommonWork(wsId, person)
  const [open, setOpen] = useState<'groups' | 'files' | null>(null)

  if (settled(groups) && settled(files) && groups.items.length === 0 && files.items.length === 0) return null

  const groupRow = groups.loading ? <Skeleton />
    : groups.failed ? <Failed text="Không tải được nhóm chung" onRetry={groups.retry} />
      : groups.items.length > 0 ? (
        <Row
          icon={<MessageSquare size={18} strokeWidth={1.75} />}
          title={`${groups.items.length} nhóm chung`}
          sub={groups.items.map((g) => g.name).join(' · ')}
          onPress={() => setOpen('groups')}
        />
      ) : null
  const fileRow = files.loading ? <Skeleton />
    : files.failed ? <Failed text="Không tải được tệp đã chia sẻ" onRetry={files.retry} />
      : files.items.length > 0 ? (
        <Row
          icon={<FileText size={18} strokeWidth={1.75} />}
          title="Tệp đã chia sẻ với bạn"
          sub={`${files.items.length} tệp`}
          onPress={() => setOpen('files')}
        />
      ) : null

  return (
    <section aria-label="Đã cùng làm việc" className="grid gap-1 px-4 pt-2 pb-4">
      <h3 className="m-0 text-label-caps text-ink-muted">Đã cùng làm việc</h3>
      <div className="grid -mx-3">
        {groupRow}
        {fileRow}
      </div>
      {open === 'groups' && (
        <NavSheet label="Nhóm chung" onClose={() => setOpen(null)}>
          {groups.items.map((g) => (
            <SheetItem
              key={g.id}
              icon={<Avatar name={g.name} hueKey={g.id} src={g.avatar_url || undefined} shape="square" size={24} />}
              onClick={() => { setOpen(null); onOpenGroup(g.id) }}
            >
              {g.name}
            </SheetItem>
          ))}
        </NavSheet>
      )}
      {open === 'files' && (
        <NavSheet label="Tệp đã chia sẻ với bạn" onClose={() => setOpen(null)}>
          {files.items.map((f) => (
            <SheetItem
              key={f.id}
              icon={f.item_type === 'folder' ? <Folder size={18} strokeWidth={1.75} /> : <FileText size={18} strokeWidth={1.75} />}
              onClick={() => { setOpen(null); onOpenFile(f) }}
            >
              {f.name}
            </SheetItem>
          ))}
        </NavSheet>
      )}
    </section>
  )
}
