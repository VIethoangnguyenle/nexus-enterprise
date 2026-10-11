import { useEditor, EditorContent } from '@tiptap/react'
import { useEffect, useRef, useState, useCallback } from 'react'
import { Paperclip, Smile, SendHorizontal, Check } from 'lucide-react'
import { EmojiPicker } from './EmojiPicker'
import { MentionDropdown } from './MentionDropdown'
import { FormattingToolbar } from './FormattingToolbar'
import { LinkDialog } from './LinkDialog'
import { chatExtensions, mentionsOf } from './chat-extensions'
import { Button, IconButton, Spinner, toast } from '../primitives'
import { useChannelMembers } from '../../hooks/useMessaging'
import { useAuthStore } from '../../stores/auth.store'
import { displayName, EMPTY_DIRECTORY, type PeopleDirectory } from '../../lib/people'

/** A message opened for editing in the composer (phones): the bar, the text, "Lưu" in place of "Gửi". */
export interface ComposerEdit {
  initialHtml: string
  /** The text now in the box; `empty` when it holds none, which the owner turns into a question about deleting. */
  onSave: (html: string, empty: boolean) => void
  onCancel: () => void
}

interface ChatEditorProps {
  /** `mentions` are the user ids of the people @-mentioned in the text. */
  onSend: (content: string, mentions: string[]) => void
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
}: ChatEditorProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const boxRef = useRef<HTMLDivElement>(null)
  const userId = useAuthStore((s) => s.user?.id)
  const [uploading, setUploading] = useState(false)
  const [showEmoji, setShowEmoji] = useState(false)
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const [empty, setEmpty] = useState(true)
  const [barOpen, setBarOpen] = useState(() => readBar(userId))
  const [link, setLink] = useState<LinkState | null>(null)
  const { data: membersData } = useChannelMembers(channelId)
  const barId = `fmt-${channelId}-${tone}`

  const editing = !!edit
  const editRef = useRef(edit)
  editRef.current = edit
  const onEditLastRef = useRef(onEditLast)
  onEditLastRef.current = onEditLast
  const openLinkRef = useRef<() => void>(() => {})

  const editor = useEditor({
    extensions: chatExtensions(placeholder),
    editorProps: {
      attributes: {
        class: 'chat-editor-content',
        'aria-label': placeholder,
        role: 'textbox',
        'aria-multiline': 'true',
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
        if (mod && !event.shiftKey && event.key.toLowerCase() === 'k') {
          event.preventDefault()
          openLinkRef.current()
          return true
        }
        if (event.key === 'ArrowUp' && !mod && !event.shiftKey && onEditLastRef.current && !editRef.current && view.state.doc.textContent === '' && !view.state.doc.firstChild?.isAtom && view.state.doc.childCount === 1) {
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
      if (!editRef.current) onTyping?.()
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

  const handleSend = useCallback(() => {
    if (!editor) return
    if (editRef.current) {
      editRef.current.onSave(editor.isEmpty ? '' : editor.getHTML(), editor.isEmpty)
      return
    }
    if (editor.isEmpty) return
    onSend(editor.getHTML(), mentionsOf(editor))
    editor.commands.clearContent()
    setEmpty(true)
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

  const handleFileSelect = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file || !onFileUpload) return
    setUploading(true)
    try {
      await onFileUpload(file)
    } catch {
      toast.error(`Chưa gửi được “${file.name}”. Kiểm tra kết nối rồi thử lại.`)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className={`relative ${tone === 'raised' ? 'mx-4 mb-4' : 'mx-3 mb-3'}`}>
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
      <div
        ref={boxRef}
        className={`grid gap-2 py-2.5 pr-2.5 pl-3.5 rounded-lg focus-within:field-focus
          ${tone === 'raised' ? 'bg-raised' : 'bg-base'}`}
      >
        {barOpen && <FormattingToolbar editor={editor} id={barId} onLink={(anchor) => openLink(anchor)} />}
        <div className="min-w-0 max-h-50 overflow-y-auto py-1" onClick={() => editor?.commands.focus()}>
          <EditorContent editor={editor} />
        </div>
        <div className="flex items-center gap-0.5">
          {onFileUpload && !editing && (
            <>
              {/* eslint-disable-next-line no-restricted-syntax -- Ô chọn tệp luôn ẩn, chỉ mở hộp
                  thoại hệ thống qua fileRef.current.click(); không có mặt thị giác để primitive tạo hình. */}
              <input ref={fileRef} type="file" className="hidden" onChange={handleFileSelect} tabIndex={-1} />
              <IconButton
                onClick={() => fileRef.current?.click()}
                disabled={uploading}
                title="Đính kèm"
                aria-label="Đính kèm"
                className="max-md:w-11 max-md:h-11"
              >
                {uploading ? <Spinner size="sm" /> : <Paperclip size={18} strokeWidth={1.75} />}
              </IconButton>
            </>
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
            <span aria-hidden="true" className="text-base font-semibold underline leading-none">A</span>
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
          <Button
            variant="primary"
            size="sm"
            onClick={handleSend}
            disabled={editing ? false : isPending || empty}
            className="ml-auto shrink-0 max-md:min-h-11"
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
          <EmojiPicker onSelect={handleEmojiSelect} onClose={() => setShowEmoji(false)} />
        </div>
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
