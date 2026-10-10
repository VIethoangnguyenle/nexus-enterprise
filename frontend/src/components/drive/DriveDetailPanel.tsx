import { useState } from 'react'
import { Download, Share2 } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { useDownloadFile, useDownloadUrl } from '../../hooks/useDownloadUrl'
import { useDriveShares } from '../../hooks/useDrive'
import { formatBytes, formatDateTime, formatRelative } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Avatar, Button, PersonChip } from '../primitives'
import { SidePanel } from '../spaces/SidePanel'
import { kindOf, ownerOf, sharePermissionLabel, shareTargetName } from './drive-model'
import { SharePermissionSelect } from './SharePermissionSelect'
import { isImageFile } from '../patterns/ImagePreviewCard'

interface DriveDetailPanelProps {
  item: DriveItem
  perms: ObjectPerms
  people: PeopleDirectory
  onClose: () => void
  onShare: (item: DriveItem) => void
}

/**
 * Right panel of Tài liệu (mockup §2): what the item is, who may open it, and
 * what happened to it. Only people who may share an item can see its shares.
 */
export function DriveDetailPanel({ item, perms, people, onClose, onShare }: DriveDetailPanelProps) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const Icon = kind.icon
  const owner = ownerOf(item, people)
  const { download, isDownloading } = useDownloadFile()
  const shares = useDriveShares(item.id, perms.share)

  const sub = folder ? kind.label : `${kind.label} · ${formatBytes(item.size_bytes)}`

  return (
    <SidePanel
      label="Chi tiết tệp"
      title={item.name}
      sub={sub}
      closeLabel="chi tiết"
      onClose={onClose}
      actions={
        !folder ? (
          <Button variant="soft" size="sm" loading={isDownloading} onClick={() => void download(item)}>
            <Download size={16} strokeWidth={1.75} aria-hidden="true" />
            Tải xuống
          </Button>
        ) : undefined
      }
    >
      <div className="grid gap-5 px-3 pt-1 pb-3">
        <div className="flex items-center gap-3">
          <span className={`grid place-items-center w-10 h-10 rounded-surface shrink-0 ${kind.tile}`} aria-hidden="true">
            <Icon size={20} strokeWidth={1.75} />
          </span>
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-small m-0 min-w-0">
            <dt className="text-ink-muted">Tạo lúc</dt>
            <dd className="m-0 tnum">{formatDateTime(item.created_at)}</dd>
            <dt className="text-ink-muted">Sửa lúc</dt>
            <dd className="m-0 tnum">{formatDateTime(item.updated_at)}</dd>
          </dl>
        </div>

        {!folder && isImageFile(item.name) && <ImagePreview item={item} />}

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
          {perms.share && (
            <Button variant="soft" size="sm" className="justify-self-start" onClick={() => onShare(item)}>
              <Share2 size={16} strokeWidth={1.75} aria-hidden="true" />
              Chia sẻ
            </Button>
          )}
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

function ImagePreview({ item }: { item: DriveItem }) {
  const url = useDownloadUrl(item.id)
  const [broken, setBroken] = useState(false)
  if (url.isError || broken) return null
  if (!url.data) return <div className="skeleton h-40 rounded-surface" aria-busy="true" />
  return (
    <img
      src={url.data}
      alt={`Xem trước ${item.name}`}
      loading="lazy"
      onError={() => setBroken(true)}
      className="w-full max-h-72 object-contain rounded-surface bg-sunk"
    />
  )
}
