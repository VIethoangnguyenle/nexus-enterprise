import { useCallback, useState } from 'react'
import type { Message } from '../api/messaging'
import { explain } from '../lib/errors'
import { messageAbilities, toEditorHtml, type MessageAbilities } from '../lib/message-model'
import { toast } from '../components/primitives'
import { useCanManageChannel } from './useCanManageChannel'
import { useDeleteMessage, useEditMessage, useMarkUnread } from './useMessageLifecycle'

/** The message being edited, and what the sender had typed when a save was refused. */
export interface EditingState { message: Message; draft?: string }

export interface MessageLifecycle {
  me: string | undefined
  /** What the reader may do with a message (edit, delete, mark unread). */
  abilities: (m: Message) => MessageAbilities
  editing: EditingState | null
  startEdit: (m: Message) => void
  cancelEdit: () => void
  /** Saves the open editor's text; `empty` says the editor holds no text, which asks to delete instead. */
  saveEdit: (html: string, empty?: boolean) => void
  deleting: Message | null
  deleteBusy: boolean
  askDelete: (m: Message) => void
  cancelDelete: () => void
  confirmDelete: () => void
  markUnread: (m: Message) => void
}

/**
 * The state behind editing, deleting and un-reading messages of one
 * conversation (or one thread): which message is open for editing, which one a
 * confirmation is about, and the requests, with a spoken reason when the
 * server refuses (DESIGN.md "Sửa tin", "Xoá tin"). The menus, the sheet and the
 * dialog only read this.
 */
export function useMessageActions(channelId: string, me: string | undefined): MessageLifecycle {
  const canManage = useCanManageChannel(channelId)
  const edit = useEditMessage(channelId)
  const remove = useDeleteMessage(channelId)
  const unread = useMarkUnread(channelId)
  const [editing, setEditing] = useState<EditingState | null>(null)
  const [deleting, setDeleting] = useState<Message | null>(null)

  const abilities = useCallback((m: Message) => messageAbilities(m, { me, canManage }), [me, canManage])

  const startEdit = (m: Message) => {
    if (abilities(m).edit) setEditing({ message: m })
  }
  const cancelEdit = () => setEditing(null)

  const saveEdit = (html: string, empty = false) => {
    if (!editing) return
    const { message } = editing
    if (empty) {
      // Saving no text at all is a delete: the editor stays until the answer to the question.
      setDeleting(message)
      return
    }
    setEditing(null)
    if (html === toEditorHtml(message)) return
    edit.mutate(
      { message, content: html },
      {
        onError: (err) => {
          toast.error(explain(err, 'sửa tin'))
          setEditing({ message, draft: html })
        },
      },
    )
  }

  const askDelete = (m: Message) => {
    if (abilities(m).remove) setDeleting(m)
  }
  const cancelDelete = () => setDeleting(null)
  const confirmDelete = () => {
    if (!deleting) return
    const target = deleting
    remove.mutate(target, {
      onSuccess: () => {
        setDeleting(null)
        setEditing((e) => (e?.message.id === target.id ? null : e))
      },
      onError: (err) => toast.error(explain(err, 'xoá tin')),
    })
  }

  const markUnread = (m: Message) => {
    unread.mutate(m, {
      onSuccess: () => toast('Đã đánh dấu chưa đọc'),
      onError: (err) => toast.error(explain(err, 'đánh dấu chưa đọc')),
    })
  }

  return {
    me, abilities, editing, startEdit, cancelEdit, saveEdit,
    deleting, deleteBusy: remove.isPending, askDelete, cancelDelete, confirmDelete, markUnread,
  }
}
