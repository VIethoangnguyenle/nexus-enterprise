import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Check, ChevronDown, ChevronRight, CircleAlert, Lock, MoreHorizontal, Trash2, WifiOff } from 'lucide-react'
import type { TextDocument } from '../../api/documents'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useDocumentSave } from '../../hooks/useDocumentSave'
import { invalidateTextLists, useDeleteTextDocument, useSaveTextDocument, useTextDocument } from '../../hooks/useDocuments'
import { type DriveSearch, textsSearch } from '../../lib/drive-search'
import { formatDateTime } from '../../lib/format'
import { statusOf } from '../../lib/errors'
import { Button, Heading, IconButton, MenuItem, MenuSeparator, PersonChip, Popover, Pressable, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { EmptyState } from '../spaces/EmptyState'
import { StatusPill } from '../approval/StatusPill'
import { ConflictDialog } from './ConflictDialog'
import { DocumentEditor, type DocumentEditorHandle } from './DocumentEditor'
import { STATUSES, saveStateLabel, statusPill, titleOf } from './document-model'

const BACK_TO_LIST = {
  to: '/drive' as const,
  search: (prev: DriveSearch) => textsSearch(prev),
}

/**
 * One document, for reading or for writing (design/mockups/contacts-documents-settings.html §3).
 * The document is fetched fresh on every visit; the page below is keyed by the
 * document, so moving to another one starts from clean state.
 *
 * Writing here is one person at a time. Saving is automatic and optimistic:
 * if someone else saved in between, nothing is overwritten and the person is
 * asked which version to keep. Live co-editing with cursors is a later phase,
 * so nothing here pretends to show other people typing.
 */
export function DocumentScreen() {
  const { docId } = useParams({ strict: false }) as { docId?: string }
  const q = useTextDocument(docId ?? '')

  if (q.isError) {
    const s = statusOf(q.error)
    const gone = s === 403 || s === 404
    return (
      <Frame>
        <EmptyState
          icon={gone ? <Lock size={24} strokeWidth={1.75} /> : <CircleAlert size={24} strokeWidth={1.75} />}
          text={
            gone
              ? 'Không mở được văn bản này. Có thể nó đã bị xoá hoặc bạn chưa có quyền xem. Hỏi chủ sở hữu hoặc quản trị viên.'
              : 'Không tải được văn bản. Kiểm tra kết nối rồi thử lại.'
          }
          action={
            gone ? (
              <Link {...BACK_TO_LIST} className="inline-flex items-center h-8 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink no-underline hover:bg-line focus-ring">
                Về danh sách văn bản
              </Link>
            ) : (
              <Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>
            )
          }
        />
      </Frame>
    )
  }
  if (!q.data) {
    return (
      <Frame>
        <div aria-busy="true" className="mx-auto w-full max-w-180 grid gap-3 px-5 pt-6">
          <span className="skeleton h-7 w-2/3 rounded-sm" />
          <span className="skeleton h-9 rounded-surface" />
          <span className="skeleton h-96 rounded-surface" />
        </div>
      </Frame>
    )
  }
  return <DocumentPage key={q.data.id} doc={q.data} />
}

function Frame({ children }: { children: ReactNode }) {
  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Văn bản">
      <div className="flex-1 grid content-center">{children}</div>
    </section>
  )
}

function DocumentPage({ doc }: { doc: TextDocument }) {
  const navigate = useNavigate()
  const { workspaceId: wsId } = useActiveWorkspace()
  const mutation = useSaveTextDocument(doc.id)
  const remove = useDeleteTextDocument(wsId)
  const editor = useRef<DocumentEditorHandle>(null)
  const canWrite = doc.can_write === true

  const s = useDocumentSave({
    initial: { version: doc.version, title: doc.title, content: doc.content ?? '', status: doc.status },
    save: (body) => mutation.mutateAsync(body),
    // Said when someone leaves with text that then could not be saved: the page
    // is gone, a toast is the only way left to tell them.
    onUnsaved: (reason) =>
      toast.error(
        reason === 'conflict'
          ? `Chưa lưu được phần bạn vừa viết trong “${titleOf({ title: doc.title })}” vì người khác đã lưu bản mới hơn. Mở lại văn bản và viết lại nếu cần.`
          : reason === 'offline'
            ? `Phần bạn vừa viết trong “${titleOf({ title: doc.title })}” chưa lưu được vì mất kết nối.`
            : `Phần bạn vừa viết trong “${titleOf({ title: doc.title })}” chưa lưu được. Mở lại văn bản để kiểm tra.`,
      ),
  })

  const [title, setTitle] = useState(doc.title)
  const [conflictOpen, setConflictOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [statusMenu, setStatusMenu] = useState(false)
  const [moreMenu, setMoreMenu] = useState(false)
  const statusAnchor = useRef<HTMLButtonElement>(null)
  const moreAnchor = useRef<HTMLButtonElement>(null)

  // A conflict opens its dialog by itself; closing it leaves a banner to reopen it.
  useEffect(() => { if (s.conflict) setConflictOpen(true) }, [s.conflict])
  // The lists order by last edit, so a save makes them stale.
  useEffect(() => { if (s.savedAt && wsId) void invalidateTextLists(wsId) }, [s.savedAt, wsId])

  const takeTheirs = () => {
    const theirs = s.takeTheirs()
    if (!theirs) return
    editor.current?.setContent(theirs.content ?? '')
    setTitle(theirs.title)
    setConflictOpen(false)
    toast('Đã tải bản mới nhất')
  }
  const keepMine = () => {
    s.keepMine()
    setConflictOpen(false)
  }

  const onDelete = async () => {
    // Saving is held while the delete is in flight and carries on if it fails:
    // a refused delete leaves the document, and the person still writing in it.
    s.pause()
    try {
      await remove.mutateAsync(doc.id)
    } catch {
      s.resume()
      return // told by the shared handler; the document stays
    }
    s.discard()
    toast(`Đã xoá “${titleOf({ title })}”`)
    void navigate({ ...BACK_TO_LIST })
  }

  const unsavedReason =
    s.state === 'offline' ? 'Chưa lưu được vì mất kết nối. Văn bản vẫn nằm trên trang này và sẽ tự lưu khi có mạng.'
    : s.state === 'error'
      ? s.errorStatus === 403 ? 'Bạn không còn quyền sửa văn bản này nên chưa lưu được. Sao chép phần đã viết nếu cần.'
      : s.errorStatus === 400 ? 'Chưa lưu được vì nội dung chưa hợp lệ. Kiểm tra tiêu đề và độ dài rồi sửa tiếp.'
      : 'Máy chủ đang gặp sự cố nên chưa lưu được. Văn bản vẫn nằm trên trang này; sửa tiếp hoặc thử lại.'
    : null

  const pill = statusPill(s.status)

  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Văn bản">
      <header className="px-5 pt-4 pb-3">
        <div className="mx-auto w-full max-w-180 grid gap-2">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0 text-sm">
            <nav aria-label="Đường dẫn" className="flex items-center gap-1.5 min-w-40 flex-1 text-ink-muted">
              <Link {...BACK_TO_LIST} className="shrink-0 whitespace-nowrap text-ink-muted no-underline hover:text-ink hover:underline focus-ring rounded-sm">
                Văn bản
              </Link>
              <ChevronRight size={14} strokeWidth={1.75} aria-hidden="true" />
              <span aria-current="page" className="truncate">{titleOf({ title })}</span>
            </nav>
            <span role="status" aria-live="polite" className={`shrink-0 text-xs tnum ${
              s.state === 'saved' || s.state === 'saving' ? 'text-ink-muted' : 'font-semibold text-warning'}`}>
              {canWrite ? saveStateLabel(s.state, s.savedAt) : ''}
            </span>
            {canWrite && (
              <div className="flex items-center gap-1 shrink-0">
                <Pressable
                  ref={statusAnchor}
                  aria-haspopup="menu"
                  aria-expanded={statusMenu}
                  aria-label={`Trạng thái: ${pill.label}`}
                  onClick={() => setStatusMenu((o) => !o)}
                  className="inline-flex items-center gap-1 rounded-full"
                >
                  <StatusPill pill={pill} />
                  <ChevronDown size={14} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />
                </Pressable>
                <Popover open={statusMenu} onClose={() => setStatusMenu(false)} anchorRef={statusAnchor} role="menu" label="Trạng thái văn bản" placement="bottom-end">
                  {STATUSES.map((st) => (
                    <MenuItem
                      key={st}
                      onClick={() => s.setStatus(st)}
                      icon={s.status === st ? <Check size={16} strokeWidth={1.75} /> : <span className="w-4" />}
                    >
                      {statusPill(st).label}
                    </MenuItem>
                  ))}
                </Popover>
                <IconButton ref={moreAnchor} aria-label="Thao tác với văn bản" aria-haspopup="menu" aria-expanded={moreMenu} onClick={() => setMoreMenu((o) => !o)}>
                  <MoreHorizontal size={18} strokeWidth={1.75} />
                </IconButton>
                <Popover open={moreMenu} onClose={() => setMoreMenu(false)} anchorRef={moreAnchor} role="menu" label="Thao tác với văn bản" placement="bottom-end">
                  <MenuSeparator />
                  <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={() => setConfirmDelete(true)}>
                    Xoá văn bản
                  </MenuItem>
                </Popover>
              </div>
            )}
            {!canWrite && <StatusPill pill={pill} />}
          </div>

          {canWrite ? (
            // eslint-disable-next-line no-restricted-syntax -- The document's title, edited in place at display size; TextField is a boxed 16px form field with a label above.
            <input
              aria-label="Tiêu đề văn bản"
              value={title}
              maxLength={200}
              placeholder="Văn bản chưa đặt tên"
              onChange={(e) => { setTitle(e.target.value); s.setTitle(e.target.value) }}
              onBlur={() => { if (!title.trim()) { setTitle(doc.title); s.setTitle(doc.title) } }}
              onKeyDown={(e) => {
                if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
                  e.preventDefault()
                  void s.flush()
                }
              }}
              className="w-full min-w-0 bg-transparent border-none outline-none font-display text-xl font-bold text-ink
                placeholder:text-ink-muted rounded-sm focus-visible:outline-2 focus-visible:outline-focus"
            />
          ) : (
            <Heading as="h1" look="page" className="break-words">{titleOf({ title })}</Heading>
          )}

          <div className="flex items-center gap-2 text-xs text-ink-muted">
            <PersonChip name={doc.owner_name || 'Thành viên'} hueKey={doc.owner_id} />
            <span title={formatDateTime(s.savedAt ?? doc.updated_at)}>· Sửa lần cuối {formatDateTime(s.savedAt ?? doc.updated_at)}</span>
          </div>
        </div>
      </header>

      <div className="flex-1 min-h-0 overflow-y-auto px-5 pb-8">
        <div className="mx-auto w-full max-w-180 grid gap-3">
          {!canWrite && (
            <div role="note" className="flex items-center gap-2.5 px-3 py-2 rounded-surface bg-sunk text-sm text-ink-muted">
              <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
              Bạn chỉ có quyền xem văn bản này.
            </div>
          )}
          {unsavedReason && (
            <div role="alert" className="flex items-center gap-2.5 px-3 py-2 rounded-surface bg-warning-wash text-sm">
              <WifiOff size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
              <span className="flex-1">{unsavedReason}</span>
              {s.state === 'error' && <Button size="sm" variant="soft" onClick={() => void s.flush()}>Thử lại</Button>}
            </div>
          )}
          {s.conflict && (
            <div role="alert" className="flex items-center gap-2.5 px-3 py-2 rounded-surface bg-warning-wash text-sm">
              <CircleAlert size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
              <span className="flex-1">Có bản mới hơn của văn bản này. Thay đổi của bạn chưa được lưu.</span>
              <Button size="sm" variant="soft" onClick={() => setConflictOpen(true)}>Chọn bản giữ lại</Button>
            </div>
          )}

          <DocumentEditor
            ref={editor}
            initialHtml={doc.content ?? ''}
            editable={canWrite}
            label={`Nội dung ${titleOf({ title })}`}
            onChange={s.setContent}
            onSaveShortcut={() => void s.flush()}
          />
        </div>
      </div>

      {s.conflict && (
        <ConflictDialog
          open={conflictOpen}
          onClose={() => setConflictOpen(false)}
          mine={s.getLocal()}
          theirs={s.conflict}
          onTakeTheirs={takeTheirs}
          onKeepMine={keepMine}
        />
      )}
      <ConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={() => void onDelete()}
        title="Xoá văn bản"
        description={
          <>
            Xoá <b className="font-semibold text-ink">{titleOf({ title })}</b>? Văn bản biến mất với mọi người và
            không hoàn tác được.
          </>
        }
        confirmLabel="Xoá"
        confirmVariant="danger"
        loading={remove.isPending}
      />
    </section>
  )
}
