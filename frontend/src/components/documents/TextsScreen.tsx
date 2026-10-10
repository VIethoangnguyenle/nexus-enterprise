import { useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { CircleAlert, FileText, Plus } from 'lucide-react'
import type { TextDocument } from '../../api/documents'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCreateTextDocument, useDeleteTextDocument, useTextDocuments } from '../../hooks/useDocuments'
import { usePeople } from '../../hooks/usePeople'
import { folderSearch, textsSearch, type DriveSearch } from '../../lib/drive-search'
import { normalize } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, FilterChip, Heading, SearchField, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { DriveTree } from '../drive/DriveTree'
import { EmptyState } from '../spaces/EmptyState'
import { TextsTable } from './TextsTable'
import { GROUPS, groupOf, titleOf } from './document-model'

/**
 * Văn bản, the group at the top of Tài liệu's list panel (mockup §2): the
 * documents written in the app, in the same hairline table as files. The list
 * panel is the one Tài liệu has; the group opened is part of the URL.
 */
export function TextsScreen() {
  const search = useSearch({ strict: false }) as DriveSearch
  const navigate = useNavigate()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = usePeople(wsId)
  const group = groupOf(search.group)
  const q = useTextDocuments(wsId, group.scope)
  const create = useCreateTextDocument(wsId)
  const remove = useDeleteTextDocument(wsId)

  const [query, setQuery] = useState('')
  const [toDelete, setToDelete] = useState<TextDocument | null>(null)

  const docs = useMemo(() => {
    const needle = normalize(query)
    return (q.docs ?? []).filter((d) => !needle || normalize(titleOf(d)).includes(needle))
  }, [q.docs, query])
  const loading = !q.docs && !q.isError
  const total = q.docs?.length ?? 0
  const more = q.hasNextPage
  const workspaceLabel = workspaceDisplayName(workspaceName)

  const newDocument = async () => {
    try {
      const doc = await create.mutateAsync({})
      void navigate({ to: '/documents/$docId', params: { docId: doc.id } })
    } catch {
      // Told by the shared handler.
    }
  }

  const confirmDelete = async () => {
    const doc = toDelete
    if (!doc) return
    try {
      await remove.mutateAsync(doc.id)
    } catch {
      return // told by the shared handler; the dialog stays for another try
    }
    setToDelete(null)
    toast(`Đã xoá “${titleOf(doc)}”`)
  }

  const body = (() => {
    if (q.isError && !q.docs) {
      return (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được danh sách văn bản. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>}
        />
      )
    }
    if (!loading && total === 0) {
      return (
        <EmptyState
          icon={<FileText size={24} strokeWidth={1.75} />}
          text={
            group.id === 'drafts' ? 'Bạn chưa có bản nháp nào. Bắt đầu một văn bản mới, nó tự lưu khi bạn viết.'
            : group.id === 'shared' ? 'Chưa có ai chia sẻ văn bản với bạn. Văn bản của người khác mà bạn được xem sẽ hiện ở đây.'
            : 'Chưa có văn bản nào. Viết văn bản đầu tiên của workspace này, mọi người có quyền sẽ thấy ngay.'
          }
          action={
            group.id === 'shared' ? undefined : (
              <Button size="sm" loading={create.isPending} onClick={() => void newDocument()}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                Văn bản mới
              </Button>
            )
          }
        />
      )
    }
    if (!loading && docs.length === 0) {
      return (
        <EmptyState
          icon={<FileText size={24} strokeWidth={1.75} />}
          text="Không có văn bản nào khớp."
          action={<Button variant="soft" size="sm" onClick={() => setQuery('')}>Xoá tìm kiếm</Button>}
        />
      )
    }
    return (
      <>
        <TextsTable
          label={`Văn bản: ${group.title}`}
          docs={docs}
          loading={loading}
          people={people}
          scope={`${wsId}:${group.id}`}
          onDelete={setToDelete}
        />
        {more && (
          <div className="px-5 pb-6">
            <Button variant="soft" size="sm" loading={q.isFetchingNextPage} onClick={() => void q.fetchNextPage()}>
              Tải thêm
            </Button>
          </div>
        )}
      </>
    )
  })()

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <div className="hidden lg:block w-70 shrink-0 min-h-0">
        <DriveTree
          workspaceId={wsId}
          workspaceLabel={workspaceLabel}
          trail={[]}
          sharedActive={false}
          textsActive={group.id}
          query={query}
          onQuery={setQuery}
          searchLabel="Tìm văn bản"
          onOpen={(folderId) => void navigate({ to: '/drive', search: (prev: DriveSearch) => {
            const { view: _v, group: _g, ...rest } = prev
            return folderId ? { ...rest, folder: folderId } : rest
          } })}
        />
      </div>

      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Văn bản">
        <header className="flex items-center gap-4 px-5 pt-4 pb-3 min-w-0">
          <div className="grid gap-0.5 min-w-0">
            <Heading as="h1" look="panel" className="truncate">{group.title}</Heading>
            <span className="text-sm text-ink-muted truncate">
              {loading ? 'Đang tải…' : `${total}${more ? '+' : ''} văn bản bạn được xem`}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-2 shrink-0">
            <Button size="sm" loading={create.isPending} onClick={() => void newDocument()}>
              <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
              Văn bản mới
            </Button>
          </div>
        </header>

        {/* The list panel is drawn from lg up; below that its groups are chips and its search sits here. */}
        <div className="flex flex-wrap items-center gap-2 px-5 pb-3 lg:hidden">
          <FilterChip pressed={false} onClick={() => void navigate({ to: '/drive', search: (p: DriveSearch) => folderSearch(p) })}>
            Tất cả tệp
          </FilterChip>
          {GROUPS.map((g) => (
            <FilterChip
              key={g.id}
              pressed={group.id === g.id}
              onClick={() => void navigate({ to: '/drive', search: (p: DriveSearch) => textsSearch(p, g.id === 'all' ? undefined : g.id) })}
            >
              {g.id === 'all' ? 'Văn bản' : g.label}
            </FilterChip>
          ))}
          <SearchField label="Tìm văn bản" tone="sunk" value={query} onChange={(e) => setQuery(e.target.value)} className="w-full sm:ml-auto sm:w-64" />
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto">{body}</div>
      </section>

      <ConfirmDialog
        open={!!toDelete}
        onClose={() => setToDelete(null)}
        onConfirm={() => void confirmDelete()}
        title="Xoá văn bản"
        description={
          <>
            Xoá <b className="font-semibold text-ink">{toDelete ? titleOf(toDelete) : ''}</b>? Văn bản biến mất với mọi
            người và không hoàn tác được.
          </>
        }
        confirmLabel="Xoá"
        confirmVariant="danger"
        loading={remove.isPending}
      />
    </div>
  )
}
