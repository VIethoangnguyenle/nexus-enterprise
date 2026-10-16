import { Download, Share2 } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { useDriveShares } from '../../hooks/useDrive'
import { formatBytes, formatDateTime, formatRelative } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Avatar, Button, PersonChip } from '../primitives'
import { SidePanel } from '../spaces/SidePanel'
import { FileThumb } from '../files/FileThumb'
import { itemVersion, kindOf, ownerOf, sharePermissionLabel, shareTargetName } from './drive-model'
import { SharePermissionSelect } from './SharePermissionSelect'

interface DriveDetailPanelProps {
  item: DriveItem
  perms: ObjectPerms
  people: PeopleDirectory
  /** Name of the folder the item is in; unknown in the shared view. */
  location?: string
  onClose: () => void
  onShare: (item: DriveItem) => void
  /** Open the file in the viewer; `el` is where focus returns to. */
  onOpen?: (item: DriveItem, el?: HTMLElement) => void
}

/**
 * Right panel of Tài liệu (DESIGN.md "Tài liệu: lưới, danh sách, panel xem
 * trước"): the file's picture, Mở · Tải xuống · Chia sẻ, what it is, who may
 * open it, and what happened to it. Only people who may share an item can see
 * its shares.
 */
export function DriveDetailPanel({ item, perms, people, location, onClose, onShare, onOpen }: DriveDetailPanelProps) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const Icon = kind.icon
  const owner = ownerOf(item, people)
  const { download, isDownloading } = useDownloadFile()
  const shares = useDriveShares(item.id, perms.share)

  return (
    <SidePanel label="Chi tiết tệp" title={item.name} closeLabel="chi tiết" onClose={onClose}>
      <div className="grid gap-4 px-3 pt-1 pb-3">
        {!folder && (
          <FileThumb
            fileId={item.id}
            name={item.name}
            mimeType={item.mime_type}
            version={itemVersion(item)}
            size="panel"
            aspect="4 / 3"
            radius={10}
          />
        )}

        <div className="flex items-center gap-1.5">
          {!folder && onOpen && (
            <Button size="sm" className="flex-1 max-lg:h-11" onClick={(e) => onOpen(item, e.currentTarget)}>
              Mở
            </Button>
          )}
          {!folder && (
            <Button
              variant="soft"
              size="sm"
              className="max-lg:h-11"
              loading={isDownloading}
              onClick={() => void download(item)}
            >
              <Download size={16} strokeWidth={1.75} aria-hidden="true" />
              Tải xuống
            </Button>
          )}
          {perms.share && (
            <Button variant="soft" size="sm" className="max-lg:h-11" onClick={() => onShare(item)}>
              <Share2 size={16} strokeWidth={1.75} aria-hidden="true" />
              Chia sẻ
            </Button>
          )}
        </div>

        <dl className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-2 text-small m-0">
          <dt className="text-ink-muted">Loại</dt>
          <dd className="m-0 flex items-center gap-2 min-w-0">
            <span className={`grid place-items-center w-5 h-5 rounded-sm shrink-0 ${kind.tile}`} aria-hidden="true">
              <Icon size={12} />
            </span>
            <span className="truncate">{kind.label}</span>
          </dd>
          {!folder && (
            <>
              <dt className="text-ink-muted">Dung lượng</dt>
              <dd className="m-0 tnum">{formatBytes(item.size_bytes)}</dd>
            </>
          )}
          <dt className="text-ink-muted">Chủ sở hữu</dt>
          <dd className="m-0 min-w-0">
            <PersonChip name={owner.name} hueKey={owner.hueKey} avatarUrl={owner.avatarUrl} />
          </dd>
          <dt className="text-ink-muted">Sửa lần cuối</dt>
          <dd className="m-0 tnum">{formatDateTime(item.updated_at)}</dd>
          {location && (
            <>
              <dt className="text-ink-muted">Vị trí</dt>
              <dd className="m-0 truncate">{location}</dd>
            </>
          )}
        </dl>

        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Người có quyền</h3>
          <ul aria-label="Người có quyền" className="grid gap-2 m-0 p-0 list-none">
            <li className="flex items-center justify-between gap-2">
              <PersonChip name={owner.name} hueKey={owner.hueKey} avatarUrl={owner.avatarUrl} role="chủ sở hữu" />
            </li>
            {(shares.data ?? []).map((s) => (
              <li key={s.id} className="flex items-center justify-between gap-2">
                <PersonChip
                  name={shareTargetName(s, people)}
                  hueKey={people.byNodeId.get(s.target_ngac_id)?.userId ?? s.target_ngac_id}
                  avatarUrl={people.byNodeId.get(s.target_ngac_id)?.avatarUrl || undefined}
                />
                {perms.share ? (
                  <SharePermissionSelect itemId={item.id} share={s} personName={shareTargetName(s, people)} />
                ) : (
                  <span className="shrink-0 text-small text-ink-muted">{sharePermissionLabel(s.operations)}</span>
                )}
              </li>
            ))}
          </ul>
          {shares.isLoading && <div className="skeleton h-5 w-2/3 rounded-sm" aria-busy="true" />}
          {shares.isError && <p className="m-0 text-small text-danger">Không tải được danh sách người có quyền.</p>}
        </section>

        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Hoạt động</h3>
          <ol aria-label="Hoạt động" className="grid gap-3 m-0 p-0 list-none">
            <li className="grid grid-cols-[24px_minmax(0,1fr)] gap-2.5 items-start">
              <Avatar name={owner.name} hueKey={owner.hueKey} src={owner.avatarUrl} size={24} />
              <p className="m-0 text-sm leading-snug">
                <b className="font-semibold">{owner.name}</b> {folder ? 'đã tạo thư mục' : 'đã tải lên'}
                <time className="block text-xs text-ink-muted tnum" title={formatDateTime(item.created_at)}>
                  {formatRelative(item.created_at)}
                </time>
              </p>
            </li>
          </ol>
        </section>
      </div>
    </SidePanel>
  )
}
