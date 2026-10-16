import { useQuery, useMutation } from '@tanstack/react-query'
import { sectionsApi, type NewSectionInput } from '../api/chat-sections'
import { toast } from '../components/primitives'
import { queryClient } from '../lib/query-client'
import {
  sectionOfChannel, withMoved, withPlaced, withRenamed, withRestored, withUnplaced, withoutSection,
  type ChatSection,
} from '../lib/chat-sections'
import { keys } from './keys'
import { startupPart } from './useChatState'

/** The conversation a toast names; the id is for the call only. */
export interface ConversationRef {
  id: string
  title: string
}

const bySavedOrder = (list: ChatSection[]) => [...list].sort((a, b) => a.position - b.position)

/**
 * The user's own navigator sections in one workspace, and the edits to them.
 *
 * Every edit but create changes the cached list at once and puts it back if the
 * call fails (the default mutation handler then says what went wrong). Create
 * and rename report their failure to the caller, whose popover shows it inline
 * ("Bạn đã có mục tên này"). Delete offers Hoàn tác, which creates the section
 * again at its old position with its old conversations.
 */
export function useChatSections(wsId: string) {
  const key = keys.messaging.sections(wsId)
  const list = useQuery({
    queryKey: key,
    queryFn: () =>
      startupPart(key, wsId, (s) => bySavedOrder(s.sections ?? []), async () => bySavedOrder((await sectionsApi.list(wsId)).sections ?? [])),
    enabled: !!wsId,
  })

  const read = () => queryClient.getQueryData<ChatSection[]>(key) ?? []
  const write = (next: ChatSection[]) => queryClient.setQueryData(key, next)

  /** Applies `edit` to the cached list now; the returned context puts it back. */
  const optimistically = (edit: (s: ChatSection[]) => ChatSection[]) => async () => {
    await queryClient.cancelQueries({ queryKey: key })
    const previous = read()
    write(edit(previous))
    return { previous }
  }
  const rollBack = (_e: unknown, _v: unknown, ctx: { previous: ChatSection[] } | undefined) => {
    if (ctx) write(ctx.previous)
  }
  // Another edit still in flight would be overwritten by an older answer, so
  // only the last one to settle asks the server again.
  const settle = () => {
    if (queryClient.isMutating({ mutationKey: key }) <= 1) void queryClient.invalidateQueries({ queryKey: key })
  }
  const shared = { mutationKey: key, onSettled: settle } as const

  const create = useMutation({
    ...shared,
    meta: { silentError: true },
    mutationFn: (input: NewSectionInput) => sectionsApi.create(wsId, input),
    onSuccess: (made) => write(withRestored(read(), made)),
  })

  const rename = useMutation({
    ...shared,
    meta: { silentError: true },
    mutationFn: (v: { id: string; name: string }) => sectionsApi.rename(wsId, v.id, v.name),
    onMutate: (v) => optimistically((s) => withRenamed(s, v.id, v.name))(),
    onError: rollBack,
  })

  const move = useMutation({
    ...shared,
    meta: { action: 'đổi thứ tự mục' },
    mutationFn: (v: { id: string; dir: 'up' | 'down' }) => sectionsApi.move(wsId, v.id, v.dir),
    onMutate: (v) => optimistically((s) => withMoved(s, v.id, v.dir))(),
    onError: rollBack,
  })

  const place = useMutation({
    ...shared,
    meta: { action: 'chuyển cuộc trò chuyện vào mục' },
    mutationFn: (v: { id: string; channelId: string }) => sectionsApi.place(wsId, v.id, v.channelId),
    onMutate: (v) => optimistically((s) => withPlaced(s, v.id, v.channelId))(),
    onError: rollBack,
  })

  const unplace = useMutation({
    ...shared,
    meta: { action: 'bỏ cuộc trò chuyện khỏi mục' },
    mutationFn: (channelId: string) => sectionsApi.unplace(wsId, channelId),
    onMutate: (channelId) => optimistically((s) => withUnplaced(s, channelId))(),
    onError: rollBack,
  })

  const remove = useMutation({
    ...shared,
    meta: { action: 'xoá mục' },
    mutationFn: (id: string) => sectionsApi.remove(wsId, id),
    onMutate: (id) => optimistically((s) => withoutSection(s, id))(),
    onError: rollBack,
  })

  const restore = useMutation({
    ...shared,
    meta: { action: 'khôi phục mục' },
    mutationFn: (gone: ChatSection) =>
      sectionsApi.create(wsId, { name: gone.name, position: gone.position, channelIds: gone.channel_ids }),
    onMutate: (gone) => optimistically((s) => withRestored(s, gone))(),
    onSuccess: (made, gone) => write(withRestored(withoutSection(read(), gone.id), made)),
    onError: rollBack,
  })

  /** Hoàn tác and the like never need the caller to handle a failure: the toast already did. */
  const quietly = (p: Promise<unknown>) => p.then(() => undefined, () => undefined)

  const sections = list.data ?? []

  return {
    sections,
    isLoading: list.isLoading,
    isError: list.isError,

    /** Rejects with the server's error (409: name taken) so the popover can say so. */
    async createSection(name: string, conversation?: ConversationRef): Promise<void> {
      const clean = name.trim()
      await create.mutateAsync({ name: clean, channelIds: conversation ? [conversation.id] : undefined })
      toast(
        conversation
          ? `Đã tạo mục “${clean}” và chuyển ${conversation.title} vào`
          : `Đã tạo mục “${clean}”`,
      )
    },

    async renameSection(id: string, name: string): Promise<void> {
      const clean = name.trim()
      await rename.mutateAsync({ id, name: clean })
      toast(`Đã đổi tên mục thành “${clean}”`)
    },

    moveSection: (id: string, dir: 'up' | 'down') => quietly(move.mutateAsync({ id, dir })),

    async deleteSection(id: string): Promise<void> {
      try {
        const gone = await remove.mutateAsync(id)
        toast(
          gone.channel_ids.length > 0
            ? `Đã xoá mục “${gone.name}”, cuộc trò chuyện trở về Tin nhắn trực tiếp và Nhóm`
            : `Đã xoá mục “${gone.name}”`,
          { action: { label: 'Hoàn tác', onClick: () => void quietly(restore.mutateAsync(gone)) } },
        )
      } catch {
        // Rolled back, and the shared handler has told the person.
      }
    },

    async placeConversation(c: ConversationRef, sectionId: string): Promise<void> {
      const target = read().find((s) => s.id === sectionId)
      if (!target || target.channel_ids.includes(c.id)) return
      try {
        await place.mutateAsync({ id: sectionId, channelId: c.id })
        toast(`Đã chuyển ${c.title} vào “${target.name}”`)
      } catch {
        // Rolled back, and the shared handler has told the person.
      }
    },

    async unplaceConversation(c: ConversationRef): Promise<void> {
      const from = sectionOfChannel(read(), c.id)
      if (!from) return
      try {
        await unplace.mutateAsync(c.id)
        toast(`Đã bỏ ${c.title} khỏi “${from.name}”`)
      } catch {
        // Rolled back, and the shared handler has told the person.
      }
    },
  }
}

export type ChatSectionsApi = ReturnType<typeof useChatSections>
