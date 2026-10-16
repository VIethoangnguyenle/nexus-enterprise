import { useEditor, EditorContent } from '@tiptap/react'
import { useEffect, useRef, useState, useCallback } from 'react'
import { Paperclip, Plus, Smile, SendHorizontal, Check, Upload, Camera, Image as ImageIcon } from 'lucide-react'
import { LazyEmojiPicker } from './lazy-parts'
import { MentionDropdown } from './MentionDropdown'
import { FormattingToolbar } from './FormattingToolbar'
import { LinkDialog } from './LinkDialog'
import { chatExtensions, mentionsOf } from './chat-extensions'
import { Button, IconButton, Popover, Pressable } from '../primitives'
import { useChannelMembers } from '../../hooks/useMessaging'
import { useAuthStore } from '../../stores/auth.store'
import { displayName, EMPTY_DIRECTORY, type PeopleDirectory } from '../../lib/people'
import { usePhone } from '../../hooks/usePhone'
import { useKeyboardOpen } from '../../hooks/useKeyboardOpen'
import { AttachmentTray } from './AttachmentTray'
import { type useAttachmentUploads } from '../../hooks/useAttachmentUploads'
import { useDraftsStore } from '../../stores/drafts.store'
import { ComposerCreate } from './ComposerCreate'
import { CreateMenuItems, type CreateKind } from './CreateMenuItems'
import { QuoteStrip } from './QuoteStrip'
import { useQuoteWatch } from './use-quote-watch'

/** A message opened for editing in the composer (phones): the bar, the text, "Lưu" in place of "Gửi". */
export interface ComposerEdit {
  initialHtml: string
  /** The text now in the box; `empty` when it holds none, which the owner turns into a question about deleting. */
  onSave: (html: string, empty: boolean) => void
  onCancel: () => void
}

interface ChatEditorProps {
  /** `mentions` are the user ids of the people @-mentioned in the text. */
  onSend: (
    content: string,
    mentions: string[],
    attachments?: { drive_file_id: string; width?: number; height?: number }[],
  ) => void | Promise<unknown>
  onTyping?: () => void
  onFileUpload?: (file: File) => Promise<void>
  isPending?: boolean
  error?: string | null
  placeholder?: string
  /** Quiet line next to the send button, e.g. how topics work here. */
  hint?: string
  /** `raised` on the page; `base` inside a raised side panel. */
  tone?: 'raised' | 'base'
  channelId: string
  /** Names for the @mention list. */
  people?: PeopleDirectory
  /** Set while a message is being edited here: the composer becomes its editor. */
  edit?: ComposerEdit
  /** ↑ in an empty box: open the reader's last message for editing. */
  onEditLast?: () => void
  uploads?: ReturnType<typeof useAttachmentUploads>
  /**
   * The composer of a thread: what the "+" menu creates is posted as a reply to this root.
   * Absent in the main composer.
   */
  parentMessageId?: string
  /**
   * Which draft this box shows: the typed text and the pending quote are kept per
   * key for the session. Defaults to the conversation; a thread passes its own.
   */
  draftKey?: string
}

/** Whether the formatting bar is open is remembered per person, on this device. */
const barKey = (userId?: string) => `ngac-chat-format-bar:${userId ?? ''}`
const readBar = (userId?: string) => {
  try { return localStorage.getItem(barKey(userId)) === '1' } catch { return false }
}
const writeBar = (userId: string | undefined, on: boolean) => {
  try { localStorage.setItem(barKey(userId), on ? '1' : '0') } catch { /* private mode: it just is not remembered */ }
}

interface LinkState { anchor: HTMLElement; text: string; href: string; hasLink: boolean }

/**
 * Message composer (design/mockups/spaces.html §2): a raised box with the
 * formatting bar (when "A" is on), the editor, and a bar of attach, "A", emoji,
 * hint and "Gửi" below. Enter sends (a new line inside a list or a code block;
 * Ctrl/Cmd+Enter sends from there), Shift+Enter breaks the line, "@" opens the
 * member list, ↑ in an empty box edits your last message. With `edit` it is the
 * editor of one message: the "Đang sửa tin" bar, Enter or "Lưu" saves, Esc cancels.
 */
export function ChatEditor({
  onSend,
  onTyping,
  onFileUpload,
  isPending,
  error,
  placeholder = 'Viết tin nhắn',
  hint,
  tone = 'raised',
  channelId,
  people,
  edit,
  onEditLast,
  uploads: propUploads,
  parentMessageId,
  draftKey,
}: ChatEditorProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const cameraRef = useRef<HTMLInputElement>(null)
  const imageRef = useRef<HTMLInputElement>(null)
  const boxRef = useRef<HTMLDivElement>(null)
  const userId = useAuthStore((s) => s.user?.id)
  const phone = usePhone()
  // On a phone with the keyboard up the composer docks flush to it, like Google Chat (DESIGN.md "Ô soạn").
  const docked = useKeyboardOpen()
  const [showEmoji, setShowEmoji] = useState(false)
  const [showAttachMenu, setShowAttachMenu] = useState(false)
  const [showAttachSheet, setShowAttachSheet] = useState(false)
  const [showCreateMenu, setShowCreateMenu] = useState(false)
  const [createKind, setCreateKind] = useState<CreateKind | null>(null)
  const createRef = useRef<HTMLButtonElement>(null)
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const [empty, setEmpty] = useState(true)
  const [barOpen, setBarOpen] = useState(() => readBar(userId))
  const [link, setLink] = useState<LinkState | null>(null)
  const { data: membersData } = useChannelMembers(channelId)
  const barId = `fmt-${channelId}-${tone}`
  const uploads = propUploads
  const uploadsRef = useRef(uploads)
  uploadsRef.current = uploads

  const editing = !!edit
  const editRef = useRef(edit)
  editRef.current = edit

  // The draft (typed text and pending quote) of this composer is kept per conversation or thread.
  const scope = draftKey ?? channelId
  const scopeRef = useRef(scope)
  const quote = useDraftsStore((d) => d.drafts[scope]?.quote)
  const quoteRef = useRef(quote)
  quoteRef.current = quote
  useQuoteWatch(scope, channelId, quote)
  const [announce, setAnnounce] = useState('')
  const editorRef = useRef<ReturnType<typeof useEditor>>(null)
  // The lists the Esc key closes before it takes the quote away.
  const popoversOpen = useRef(false)
  popoversOpen.current = showEmoji || showAttachMenu || showCreateMenu
  const closePopovers = useRef(() => {})
  closePopovers.current = () => {
    setShowEmoji(false)
    setShowAttachMenu(false)
    setShowCreateMenu(false)
  }
  const clearQuote = useCallback(() => {
    useDraftsStore.getState().setQuote(scopeRef.current, null)
    setAnnounce('Đã bỏ trích dẫn')
    editorRef.current?.commands.focus()
  }, [])
  const clearQuoteRef = useRef(clearQuote)
  clearQuoteRef.current = clearQuote

  const onEditLastRef = useRef(onEditLast)
  onEditLastRef.current = onEditLast
  const openLinkRef = useRef<() => void>(() => {})

  const editor = useEditor({
    extensions: chatExtensions(placeholder),
    content: useDraftsStore.getState().drafts[scope]?.html ?? '',
    editorProps: {
      attributes: {
        class: 'chat-editor-content',
        'aria-label': placeholder,
        role: 'textbox',
        'aria-multiline': 'true',
      },
      handleDOMEvents: {
        paste: (_view, event) => {
          if (editRef.current) return false
          const clipboardData = event.clipboardData
          if (!clipboardData) return false

          // If clipboard contains plain text or HTML, do not intercept as image attachments.
          const plainText = typeof clipboardData.getData === 'function' ? clipboardData.getData('text/plain') : ''
          const htmlText = typeof clipboardData.getData === 'function' ? clipboardData.getData('text/html') : ''
          const hasText = Boolean(plainText?.trim() || htmlText?.trim())
          if (hasText) {
            return false
          }

          const items = clipboardData.items
          const files: File[] = []
          if (items) {
            for (let i = 0; i < items.length; i++) {
              const item = items[i]
              if (item && item.type?.startsWith('image/')) {
                const f = typeof item.getAsFile === 'function' ? item.getAsFile() : null
                if (f) files.push(f)
              }
            }
          }
          if (files.length === 0 && clipboardData.files?.length) {
            for (let i = 0; i < clipboardData.files.length; i++) {
              const f = clipboardData.files[i]
              if (f && f.type?.startsWith('image/')) files.push(f)
            }
          }
          if (files.length > 0) {
            event.preventDefault()
            uploadsRef.current?.add(files)
            return true
          }
          return false
        },
      },
      handleKeyDown: (view, event) => {
        // The mention list handles its keys in the capture phase and marks them
        // handled; swallow those so Enter neither sends nor splits the line.
        if (event.defaultPrevented) return true
        const mod = event.ctrlKey || event.metaKey
        if (event.key === 'Escape' && editRef.current) {
          event.preventDefault()
          event.stopPropagation()
          editRef.current.onCancel()
          return true
        }
        // Esc takes away one thing at a time: open lists first, then the pending quote.
        // With neither, it is left to the panel behind.
        if (event.key === 'Escape' && !mentionOpen.current) {
          if (popoversOpen.current) {
            event.preventDefault()
            closePopovers.current()
            return true
          }
          if (quoteRef.current) {
            event.preventDefault()
            clearQuoteRef.current()
            return true
          }
        }
        if (mod && !event.shiftKey && event.key.toLowerCase() === 'k') {
          event.preventDefault()
          openLinkRef.current()
          return true
        }
        if (event.key === 'ArrowUp' && !mod && !event.shiftKey && onEditLastRef.current && !editRef.current && !quoteRef.current && view.state.doc.textContent === '' && !view.state.doc.firstChild?.isAtom && view.state.doc.childCount === 1) {
          event.preventDefault()
          onEditLastRef.current()
          return true
        }
        if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
          if (mentionOpen.current && !mod) return false
          // In a list or a code block Enter makes the next line; Ctrl/Cmd+Enter sends from there.
          const inBlock = ['bulletList', 'orderedList', 'codeBlock'].some((n) => isActiveRef.current(n))
          if (inBlock && !mod) return false
          event.preventDefault()
          sendRef.current()
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor: ed }) => {
      if (!editRef.current) {
        onTyping?.()
        useDraftsStore.getState().setHtml(scopeRef.current, ed.isEmpty ? '' : ed.getHTML())
      }
      setEmpty(ed.isEmpty)
      const { from } = ed.state.selection
      const textBefore = ed.state.doc.textBetween(Math.max(0, from - 20), from, '\n', '\0')
      const mentionMatch = textBefore.match(/@(\w*)$/)
      setMentionQuery(mentionMatch ? mentionMatch[1] ?? '' : null)
    },
  // The placeholder names the conversation, which can arrive after mount;
  // TipTap reads it once, so the editor is rebuilt when it changes.
  }, [placeholder])

  const isActiveRef = useRef((name: string) => !!editor?.isActive(name))
  isActiveRef.current = (name: string) => !!editor?.isActive(name)

  const mentionOpen = useRef(false)
  mentionOpen.current = mentionQuery !== null
  editorRef.current = editor

  // A box made with a draft in it (restored on return, or rebuilt for a new placeholder) is not empty.
  useEffect(() => {
    if (editor) setEmpty(editor.isEmpty)
  }, [editor])

  // Another conversation or thread: show its draft. (A rebuilt editor reads it when it is made.)
  useEffect(() => {
    if (!editor || scopeRef.current === scope) return
    scopeRef.current = scope
    if (editRef.current) return
    editor.commands.setContent(useDraftsStore.getState().drafts[scope]?.html ?? '')
    setEmpty(editor.isEmpty)
  }, [editor, scope])

  // The menus read this to leave "Trích dẫn" out while the box is an editor (phones).
  useEffect(() => {
    if (!editing) return
    useDraftsStore.getState().setEditing(scope, true)
    return () => useDraftsStore.getState().setEditing(scope, false)
  }, [editing, scope])

  // A quote just chosen (not one restored with the draft): the box takes focus, and it is announced.
  const quotedId = quote?.id
  const seenQuote = useRef(quotedId)
  useEffect(() => {
    if (seenQuote.current === quotedId) return
    seenQuote.current = quotedId
    if (!quote || editing) return
    const own = !!userId && quote.sender_id === userId
    const who = own ? 'bạn' : displayName(people ?? EMPTY_DIRECTORY, quote.sender_id, quote.sender_name)
    setAnnounce(`Đang trích dẫn tin của ${who}. Esc để bỏ.`)
    editor?.commands.focus('end')
  }, [quotedId])
  const quoteGone = !!quote?.deleted
  useEffect(() => {
    if (quoteGone) setAnnounce('Tin bạn đang trích dẫn đã bị xoá')
  }, [quoteGone, quotedId])

  const goToQuoted = useCallback(() => {
    if (!quotedId) return
    document.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(quotedId)}"]`)?.scrollIntoView?.({ block: 'center' })
  }, [quotedId])

  // Close attach menu on outside click
  useEffect(() => {
    if (!showAttachMenu) return
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null
      if (
        !target?.closest('[role="menu"][aria-label="Đính kèm"]') &&
        !target?.closest('[aria-label="Đính kèm"]')
      ) {
        setShowAttachMenu(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [showAttachMenu])

  // Editing takes the box over: the draft is kept aside and comes back when editing ends.
  const draft = useRef('')
  const editKey = edit?.initialHtml
  useEffect(() => {
    if (!editor || editKey === undefined) return
    draft.current = editor.getHTML()
    editor.commands.setContent(editKey)
    editor.commands.focus('end')
    setEmpty(editor.isEmpty)
    return () => {
      if (editor.isDestroyed) return
      editor.commands.setContent(draft.current === '<p></p>' ? '' : draft.current)
      setEmpty(editor.isEmpty)
    }
  }, [editor, editKey])

  const canAttach = !!uploads || !!onFileUpload
  const hasUploads = (uploads?.items.length ?? 0) > 0
  const uploadingCount = uploads?.items.filter((it) => it.status === 'uploading').length ?? 0
  const errorCount = uploads?.items.filter((it) => it.status === 'error').length ?? 0
  const hasUploadingOrError = uploadingCount > 0 || errorCount > 0

  const whyStatus = uploadingCount > 0
    ? `Đang tải ${uploadingCount} tệp…`
    : errorCount > 0
      ? 'Thử lại hoặc bỏ tệp lỗi để gửi'
      : ''

  const isSendDisabled = editing
    ? false
    : isPending || hasUploadingOrError || (empty && !hasUploads)

  const handleSend = useCallback(async () => {
    if (!editor) return
    if (editRef.current) {
      editRef.current.onSave(editor.isEmpty ? '' : editor.getHTML(), editor.isEmpty)
      return
    }
    const currentUploads = uploadsRef.current
    const items = currentUploads?.items ?? []
    const hasUploadingOrError = items.some(
      (it) => it.status === 'uploading' || it.status === 'error',
    )
    if (hasUploadingOrError) return
    if (editor.isEmpty && items.length === 0) return

    const doneAttachments = items
      .filter((it) => it.status === 'done' && it.fileId)
      .map((it) => ({
        drive_file_id: it.fileId!,
        ...(it.width ? { width: it.width } : {}),
        ...(it.height ? { height: it.height } : {}),
      }))

    const content = editor.isEmpty ? '' : editor.getHTML()
    const mentions = mentionsOf(editor)

    // The text leaves the draft now, not when the server answers: the composer can be gone by then
    // (another conversation, a closed thread) and a draft left behind would be sent a second time.
    const sentScope = scopeRef.current
    const drafts = useDraftsStore.getState()
    drafts.setHtml(sentScope, '')
    editor.commands.clearContent()
    setEmpty(true)

    try {
      const res = doneAttachments.length > 0
        ? onSend(content, mentions, doneAttachments)
        : onSend(content, mentions)
      await Promise.resolve(res)
      currentUploads?.clear()
    } catch {
      // Send failed: the text and the attachments in the tray stay, so nothing is lost. The text
      // returns only to a composer that has not been given new text meanwhile.
      if (content && !useDraftsStore.getState().drafts[sentScope]?.html) {
        if (!editor.isDestroyed && scopeRef.current === sentScope) {
          editor.commands.setContent(content)
          setEmpty(editor.isEmpty)
        } else {
          useDraftsStore.getState().setHtml(sentScope, content)
        }
      }
    }
  }, [editor, onSend])
  const sendRef = useRef(handleSend)
  sendRef.current = handleSend

  const closeLink = () => {
    setLink(null)
    editor?.commands.focus()
  }
  const openLink = useCallback((anchor?: HTMLElement | null) => {
    if (!editor) return
    const { from, to } = editor.state.selection
    const hasLink = editor.isActive('link')
    if (hasLink) editor.chain().extendMarkRange('link').run()
    const range = editor.state.selection
    setLink({
      anchor: anchor ?? boxRef.current ?? document.body,
      text: editor.state.doc.textBetween(hasLink ? range.from : from, hasLink ? range.to : to, ' '),
      href: hasLink ? ((editor.getAttributes('link').href as string) ?? '') : '',
      hasLink,
    })
  }, [editor])
  openLinkRef.current = () => openLink(barOpen ? document.querySelector<HTMLElement>(`#${CSS.escape(barId)} [aria-label="Link"]`) : null)

  const submitLink = ({ text, href }: { text: string; href: string }) => {
    editor?.chain().focus().extendMarkRange('link').insertContent({ type: 'text', text, marks: [{ type: 'link', attrs: { href } }] }).run()
    setLink(null)
  }
  const removeLink = () => {
    editor?.chain().focus().extendMarkRange('link').unsetLink().run()
    setLink(null)
  }
  const anchorRef = useRef<HTMLElement | null>(null)
  anchorRef.current = link?.anchor ?? null

  const toggleBar = () => {
    const next = !barOpen
    setBarOpen(next)
    writeBar(userId, next)
  }

  const handleEmojiSelect = useCallback((emoji: string) => {
    editor?.commands.insertContent(emoji)
    setShowEmoji(false)
    editor?.commands.focus()
  }, [editor])

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    if (!files.length) return
    if (uploads) {
      uploads.add(files)
    } else if (onFileUpload) {
      for (const file of files) {
        void onFileUpload(file)
      }
    }
    if (fileRef.current) fileRef.current.value = ''
    if (cameraRef.current) cameraRef.current.value = ''
    if (imageRef.current) imageRef.current.value = ''
  }

  return (
    <div
      data-composer-root
      data-docked={docked || undefined}
      className={docked ? 'relative' : `relative ${tone === 'raised' ? 'mx-4 mb-4' : 'mx-3 mb-3'}
        max-md:mb-[max(0.75rem,env(safe-area-inset-bottom))]`}
    >
      {error && (
        <div role="alert" className="mb-2 px-3 py-2 rounded-md bg-danger-wash text-danger text-small">
          {error}
        </div>
      )}
      {editing && (
        <div className="flex items-center gap-2 mb-1.5 px-1 text-small">
          <span className="font-semibold text-accent">Đang sửa tin</span>
          <Button type="button" variant="ghost" size="sm" className="ml-auto max-md:min-h-11" onClick={edit.onCancel}>Huỷ</Button>
        </div>
      )}
      {editing && quote && (
        <p className="mb-1.5 px-1 text-xs text-ink-muted">
          Trích dẫn tin của {!!userId && quote.sender_id === userId ? 'bạn' : displayName(people ?? EMPTY_DIRECTORY, quote.sender_id, quote.sender_name)} sẽ quay lại sau khi sửa xong.
        </p>
      )}
      <p className="sr-only" aria-live="polite" data-composer-live>{announce}</p>
      <div
        ref={boxRef}
        data-composer-box
        className={docked
          ? 'grid gap-2 py-1.5 px-2.5 rounded-none border-t border-line bg-base'
          : `grid gap-2 py-2.5 pr-2.5 pl-3.5 rounded-lg focus-within:field-focus
          ${tone === 'raised' ? 'bg-raised' : 'bg-base'}`}
      >
        {quote && !editing && (
          <QuoteStrip
            quote={quote}
            people={people ?? EMPTY_DIRECTORY}
            me={userId}
            onClear={clearQuote}
            onGoTo={goToQuoted}
          />
        )}
        {barOpen && <FormattingToolbar editor={editor} id={barId} onLink={(anchor) => openLink(anchor)} />}
        {uploads && uploads.items.length > 0 && (
          <AttachmentTray items={uploads.items} onRemove={uploads.remove} onRetry={uploads.retry} />
        )}
        <div className="min-w-0 max-h-50 overflow-y-auto py-1" onClick={() => editor?.commands.focus()}>
          <EditorContent editor={editor} />
        </div>
        <div className="flex items-center gap-0.5">
          {!editing && !phone && (
            <>
              <IconButton
                ref={createRef}
                type="button"
                onClick={() => setShowCreateMenu((v) => !v)}
                title="Bình chọn, công việc, tài liệu, phê duyệt"
                aria-label="Tạo trong chat"
                aria-haspopup="menu"
                aria-expanded={showCreateMenu}
              >
                <Plus size={18} strokeWidth={1.75} />
              </IconButton>
              <Popover
                open={showCreateMenu}
                onClose={() => setShowCreateMenu(false)}
                anchorRef={createRef}
                role="menu"
                label="Tạo trong chat"
                placement="top-start"
              >
                <CreateMenuItems
                  separator={false}
                  onSelect={(k) => {
                    setShowCreateMenu(false)
                    setCreateKind(k)
                  }}
                />
              </Popover>
            </>
          )}
          {!editing && (canAttach || phone) && (
            <div className="relative">
              {/* eslint-disable-next-line no-restricted-syntax -- Ô chọn tệp luôn ẩn, chỉ mở hộp
                  thoại hệ thống qua ref.current.click(); không có mặt thị giác để primitive tạo hình. */}
              <input
                ref={fileRef}
                type="file"
                multiple
                className="hidden"
                onChange={handleFileSelect}
                tabIndex={-1}
              />
              {phone && canAttach && (
                <>
                  {/* eslint-disable-next-line no-restricted-syntax */}
                  <input
                    ref={cameraRef}
                    type="file"
                    accept="image/*"
                    capture="environment"
                    className="hidden"
                    onChange={handleFileSelect}
                    tabIndex={-1}
                  />
                  {/* eslint-disable-next-line no-restricted-syntax */}
                  <input
                    ref={imageRef}
                    type="file"
                    accept="image/*"
                    multiple
                    className="hidden"
                    onChange={handleFileSelect}
                    tabIndex={-1}
                  />
                </>
              )}
              <IconButton
                type="button"
                onClick={() => {
                  if (phone) {
                    setShowAttachSheet(true)
                  } else {
                    setShowAttachMenu((v) => !v)
                  }
                }}
                title={canAttach ? 'Đính kèm' : 'Tạo trong chat'}
                aria-label={canAttach ? 'Đính kèm' : 'Tạo trong chat'}
                aria-expanded={phone ? showAttachSheet : showAttachMenu}
                className="max-md:w-11 max-md:h-11"
              >
                {canAttach ? <Paperclip size={18} strokeWidth={1.75} /> : <Plus size={18} strokeWidth={1.75} />}
              </IconButton>

              {showAttachMenu && !phone && (
                <div
                  role="menu"
                  aria-label="Đính kèm"
                  className="absolute bottom-full left-0 mb-2 z-dropdown py-1 rounded-surface bg-overlay shadow-overlay border border-line min-w-44 flex flex-col"
                >
                  <Pressable
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setShowAttachMenu(false)
                      fileRef.current?.click()
                    }}
                    className="flex items-center gap-2.5 px-3 py-2 text-small text-ink hover:bg-hover text-left cursor-pointer"
                  >
                    <Upload size={16} strokeWidth={1.75} />
                    Tải lên từ máy
                  </Pressable>
                </div>
              )}
            </div>
          )}
          <IconButton
            type="button"
            onClick={toggleBar}
            title="Định dạng"
            aria-label="Định dạng"
            aria-pressed={barOpen}
            aria-controls={barOpen ? barId : undefined}
            className={`max-md:w-11 max-md:h-11 ${barOpen ? '!bg-accent-wash !text-accent' : ''}`}
          >
            <span aria-hidden="true" className="text-16 font-semibold underline leading-none">A</span>
          </IconButton>
          <IconButton
            onClick={() => setShowEmoji((v) => !v)}
            title="Biểu tượng cảm xúc"
            aria-label="Biểu tượng cảm xúc"
            aria-expanded={showEmoji}
            className="max-md:w-11 max-md:h-11"
          >
            <Smile size={18} strokeWidth={1.75} />
          </IconButton>
          {hint && !editing && <span className="ml-2 text-xs text-ink-muted truncate hidden md:inline">{hint}</span>}
          {whyStatus && !editing && (
            <span className="ml-auto text-xs text-ink-muted truncate mr-2" role="status">
              {whyStatus}
            </span>
          )}
          <Button
            variant="primary"
            size="sm"
            onClick={handleSend}
            disabled={isSendDisabled}
            className={`${whyStatus ? '' : 'ml-auto'} shrink-0 max-md:min-h-11`}
          >
            {editing
              ? <Check size={16} strokeWidth={1.75} aria-hidden="true" />
              : <SendHorizontal size={16} strokeWidth={1.75} aria-hidden="true" />}
            {editing ? 'Lưu' : 'Gửi'}
          </Button>
        </div>
      </div>

      {showEmoji && (
        <div className="absolute bottom-full left-0 mb-2 z-dropdown">
          <LazyEmojiPicker onSelect={handleEmojiSelect} onClose={() => setShowEmoji(false)} />
        </div>
      )}

      {showAttachSheet && phone && (
        <div className="fixed inset-0 z-modal flex flex-col justify-end">
          <div
            className="absolute inset-0 bg-scrim/40 animate-fade-in"
            onClick={() => setShowAttachSheet(false)}
            aria-hidden="true"
          />
          <div
            role="dialog"
            aria-label="Tạo và đính kèm"
            className="relative bg-overlay rounded-t-2xl p-4 pb-6 shadow-overlay z-10 flex flex-col gap-3"
          >
            <div className="w-9 h-1 rounded-full bg-line mx-auto mb-1" aria-hidden="true" />
            {canAttach && <div className="grid grid-cols-3 gap-2">
              <Pressable
                type="button"
                onClick={() => {
                  setShowAttachSheet(false)
                  cameraRef.current?.click()
                }}
                className="flex flex-col items-center justify-center gap-1.5 p-3 rounded-xl bg-base hover:bg-hover text-ink text-xs font-medium cursor-pointer"
              >
                <Camera size={22} strokeWidth={1.75} />
                Máy ảnh
              </Pressable>
              <Pressable
                type="button"
                onClick={() => {
                  setShowAttachSheet(false)
                  imageRef.current?.click()
                }}
                className="flex flex-col items-center justify-center gap-1.5 p-3 rounded-xl bg-base hover:bg-hover text-ink text-xs font-medium cursor-pointer"
              >
                <ImageIcon size={22} strokeWidth={1.75} />
                Ảnh
              </Pressable>
              <Pressable
                type="button"
                onClick={() => {
                  setShowAttachSheet(false)
                  fileRef.current?.click()
                }}
                className="flex flex-col items-center justify-center gap-1.5 p-3 rounded-xl bg-base hover:bg-hover text-ink text-xs font-medium cursor-pointer"
              >
                <Upload size={22} strokeWidth={1.75} />
                Tệp
              </Pressable>
            </div>}
            <div className="flex flex-col">
              <CreateMenuItems
                separator={canAttach}
                onSelect={(k) => {
                  setShowAttachSheet(false)
                  setCreateKind(k)
                }}
              />
            </div>
          </div>
        </div>
      )}

      {createKind && (
        <ComposerCreate
          kind={createKind}
          channelId={channelId}
          parentMessageId={parentMessageId}
          onClose={() => setCreateKind(null)}
        />
      )}

      <LinkDialog
        open={!!link}
        anchorRef={anchorRef}
        text={link?.text ?? ''}
        href={link?.href ?? ''}
        hasLink={link?.hasLink}
        onSubmit={submitLink}
        onRemove={removeLink}
        onClose={closeLink}
      />

      {mentionQuery !== null && (
        <div className="absolute bottom-full left-4 mb-1 z-dropdown">
          <MentionDropdown
            members={membersData?.members || []}
            people={people}
            query={mentionQuery}
            onSelect={(member) => {
              if (!editor) return
              const { from } = editor.state.selection
              const textBefore = editor.state.doc.textBetween(Math.max(0, from - 20), from, '\n', '\0')
              const match = textBefore.match(/@(\w*)$/)
              if (match) {
                const deleteFrom = from - match[0].length
                editor.chain()
                  .deleteRange({ from: deleteFrom, to: from })
                  .insertContent([
                    { type: 'mention', attrs: { id: member.user_id, label: displayName(people ?? EMPTY_DIRECTORY, member.user_id, member.username) } },
                    { type: 'text', text: ' ' },
                  ])
                  .focus()
                  .run()
              }
              setMentionQuery(null)
            }}
            onClose={() => setMentionQuery(null)}
          />
        </div>
      )}
    </div>
  )
}
