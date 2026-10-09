import { useMemo, useRef, type KeyboardEvent } from 'react'
import { FolderOpen, CircleAlert } from 'lucide-react'
import { useChannelDrive, useDriveFolder } from '../../hooks/useDrive'
import { driveApi, type DriveItem } from '../../api/drive'
import { fileKind } from '../../lib/file-kind'
import { formatBytes, formatDateTime, formatRelative, toMillis } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Button, PersonChip, Pressable, toast } from '../primitives'
import { EmptyState } from './EmptyState'

interface SpaceFilesProps {
  workspaceId: string
  channelId: string
  people: PeopleDirectory
  /** Switch to the conversation tab (where files are attached). */
  onGoToChat: () => void
}

/**
 * Files shared in a space, in the hairline table of Tài liệu (DESIGN.md §6
 * Table). The channel drive's root holds one folder per space; chat uploads
 * land inside it, so both levels are listed. Rows are real buttons with ↑/↓
 * between them; Enter downloads.
 */
export function SpaceFiles({ workspaceId, channelId, people, onGoToChat }: SpaceFilesProps) {
  const root = useChannelDrive(workspaceId, channelId)
  const folder = root.data?.items?.find((i) => i.item_type === 'folder')
  const inner = useDriveFolder(workspaceId, folder?.id)
  const listRef = useRef<HTMLDivElement>(null)

  const files = useMemo(() => {
    const all = [...(root.data?.items ?? []), ...(folder ? inner.data?.items ?? [] : [])]
    return all
      .filter((i) => i.item_type === 'file' && i.status === 'active')
      .sort((a, b) => toMillis(b.updated_at) - toMillis(a.updated_at))
  }, [root.data, inner.data, folder])

  const loading = root.isLoading || (!!folder && inner.isLoading)

  const download = async (item: DriveItem) => {
    try {
      const { download_url } = await driveApi.getDownloadUrl(item.id)
      const a = document.createElement('a')
      a.href = download_url
      a.download = item.name
      a.click()
    } catch {
      toast.error(`Chưa tải được “${item.name}”. Thử lại sau ít phút.`)
    }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const rows = Array.from(listRef.current?.querySelectorAll<HTMLButtonElement>('[data-row]') ?? [])
    const i = rows.indexOf(document.activeElement as HTMLButtonElement)
    const next = rows[e.key === 'ArrowDown' ? Math.min(i + 1, rows.length - 1) : Math.max(i - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  if (root.isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Chưa tải được tệp của nhóm."
        action={<Button variant="soft" size="sm" onClick={() => root.refetch()}>Thử lại</Button>}
      />
    )
  }
  if (!loading && files.length === 0) {
    return (
      <EmptyState
        icon={<FolderOpen size={24} strokeWidth={1.75} />}
        text="Chưa có tệp nào. Tệp đính kèm trong trò chuyện sẽ hiện ở đây."
        action={<Button variant="soft" size="sm" onClick={onGoToChat}>Đính kèm trong Trò chuyện</Button>}
      />
    )
  }

  const cols = 'grid grid-cols-[minmax(0,1fr)_minmax(0,180px)_120px_88px] max-lg:grid-cols-[minmax(0,1fr)_88px] gap-4 items-center px-3'
  return (
    <div className="px-5 py-2">
      <div role="row" className={`${cols} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span>Tên</span>
        <span className="max-lg:hidden">Người chia sẻ</span>
        <span className="max-lg:hidden text-right">Cập nhật</span>
        <span className="text-right">Dung lượng</span>
      </div>
      <div ref={listRef} onKeyDown={onKeyDown} aria-busy={loading || undefined}>
        {loading
          ? [0, 1, 2].map((i) => (
              <div key={i} className={`${cols} h-11 border-b border-line`}>
                <div className="skeleton h-3.5 w-2/3 rounded-sm" />
              </div>
            ))
          : files.map((f) => {
              const kind = fileKind(f.name, f.mime_type)
              const Icon = kind.icon
              const owner = people.byUserId.get(f.owner_id)
              return (
                <Pressable
                  key={f.id}
                  data-row
                  onClick={() => void download(f)}
                  aria-label={`Tải xuống ${f.name}`}
                  className={`${cols} w-full h-11 border-b border-line text-sm text-ink rounded-none
                    hover:bg-hover transition-colors duration-quick`}
                >
                  <span className="flex items-center gap-2.5 min-w-0">
                    <span className={`grid place-items-center w-7 h-7 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
                      <Icon size={16} strokeWidth={1.75} />
                    </span>
                    <span className="truncate font-medium">{f.name}</span>
                  </span>
                  <span className="max-lg:hidden min-w-0">
                    <PersonChip name={owner?.name ?? displayName(people, f.owner_id)} hueKey={f.owner_id} />
                  </span>
                  <span className="max-lg:hidden text-right text-ink-muted tnum" title={formatDateTime(f.updated_at)}>
                    {formatRelative(f.updated_at)}
                  </span>
                  <span className="text-right text-ink-muted tnum">{formatBytes(f.size_bytes)}</span>
                </Pressable>
              )
            })}
      </div>
    </div>
  )
}
