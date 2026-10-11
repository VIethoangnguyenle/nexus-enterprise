import { useState } from 'react'
import { Folder } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { driveApi } from '../../api/drive'
import { useCreateTextDocument } from '../../hooks/useDocuments'
import { useSendMessage } from '../../hooks/useMessages'
import { chatCardKeys } from '../../hooks/keys/chat-cards'
import { explain } from '../../lib/errors'
import { queryClient } from '../../lib/query-client'
import { Button, TextField } from '../primitives'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'

const TITLE_MAX = 120

/** The conversation's own folder in Tài liệu: the first folder of its drive, as attachments use it. */
const folderOptions = (wsId: string, channelId: string) => ({
  queryKey: chatCardKeys.channelFolder(wsId, channelId),
  queryFn: async () => {
    const drive = await driveApi.channelDrive(wsId, channelId)
    const folder = drive?.items?.find((i) => i.item_type === 'folder')
    return folder ? { id: folder.id, name: folder.name } : null
  },
  staleTime: 60_000,
})

interface DocDialogProps {
  open: boolean
  onClose: () => void
  workspaceId: string
  channelId: string
}

/**
 * "Tài liệu mới": a name, then a text document is made in the conversation's
 * folder and posted to it as a card. If the post fails after the document
 * exists, "Gửi lại" posts that same document instead of making another.
 */
export function DocDialog({ open, onClose, workspaceId, channelId }: DocDialogProps) {
  const create = useCreateTextDocument(workspaceId)
  const send = useSendMessage(channelId)
  return (
    <Dialog open={open} onClose={create.isPending || send.isPending ? () => {} : onClose} title="Tạo tài liệu">
      <DocForm workspaceId={workspaceId} channelId={channelId} onClose={onClose} create={create} send={send} />
    </Dialog>
  )
}

function DocForm({ workspaceId, channelId, onClose, create, send }: {
  workspaceId: string
  channelId: string
  onClose: () => void
  create: ReturnType<typeof useCreateTextDocument>
  send: ReturnType<typeof useSendMessage>
}) {
  const [title, setTitle] = useState('')
  const [titleError, setTitleError] = useState<string>()
  const [failure, setFailure] = useState<string>()
  /** The document already made, waiting to be posted. */
  const [made, setMade] = useState<string>()
  const folder = useQuery(folderOptions(workspaceId, channelId))
  const busy = create.isPending || send.isPending

  const post = async (docId: string) => {
    try {
      await send.mutateAsync({ content: '', linkedEntity: { type: 'document', id: docId } })
    } catch {
      setMade(docId)
      setFailure('Đã tạo tài liệu nhưng chưa gửi được vào nhóm. Thử gửi lại.')
      return
    }
    onClose()
  }

  const submit = async () => {
    if (!title.trim()) {
      setTitleError('Nhập tên tài liệu.')
      return
    }
    setTitleError(undefined)
    setFailure(undefined)
    if (made) return post(made)
    let doc
    try {
      const where = await queryClient.fetchQuery(folderOptions(workspaceId, channelId))
      doc = await create.mutateAsync({ title: title.trim(), ...(where ? { folder_id: where.id } : {}) })
    } catch (err) {
      setFailure(explain(err, 'tạo tài liệu'))
      return
    }
    await post(doc.id)
  }

  return (
    <form
      className="grid gap-3.5"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <TextField
        label="Tên tài liệu"
        value={title}
        maxLength={TITLE_MAX}
        counter
        autoFocus
        disabled={!!made}
        error={titleError}
        onChange={(e) => setTitle(e.target.value)}
      />
      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">Lưu vào</span>
        <div className="flex items-center gap-2 min-h-10 px-3 rounded-md bg-sunk text-sm text-ink">
          <Folder size={16} strokeWidth={1.75} className="shrink-0 text-ink-muted" aria-hidden="true" />
          <span className="truncate">{folder.data?.name ?? 'Thư mục của nhóm'}</span>
        </div>
        <small className="text-xs text-ink-muted">Mọi thành viên nhóm mở được; quyền theo thư mục của nhóm.</small>
      </div>
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={busy}>{made ? 'Đóng' : 'Huỷ'}</Button>
        <Button type="submit" loading={busy}>
          {made ? 'Gửi lại' : 'Tạo tài liệu'}
        </Button>
      </div>
    </form>
  )
}
