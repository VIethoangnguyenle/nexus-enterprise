import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { useRef, useState, useCallback } from 'react'
import { Paperclip, Smile, SendHorizontal } from 'lucide-react'
import { EmojiPicker } from './EmojiPicker'
import { MentionDropdown } from './MentionDropdown'
import { Button, IconButton, Spinner, toast } from '../primitives'
import { useChannelMembers } from '../../hooks/useMessaging'
import type { PeopleDirectory } from '../../lib/people'

interface ChatEditorProps {
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
}

/**
 * Message composer (design/mockups/spaces.html §2): a raised box with the
 * editor on top and a bar of attach, emoji, hint and "Gửi" below. Enter sends,
 * Shift+Enter breaks the line, "@" opens the member list.
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
}: ChatEditorProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [showEmoji, setShowEmoji] = useState(false)
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const [empty, setEmpty] = useState(true)
  const { data: membersData } = useChannelMembers(channelId)

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: false,
        codeBlock: { HTMLAttributes: { class: 'chat-code-block' } },
      }),
      Placeholder.configure({ placeholder }),
    ],
    editorProps: {
      attributes: {
        class: 'chat-editor-content',
        'aria-label': placeholder,
        role: 'textbox',
        'aria-multiline': 'true',
      },
      handleKeyDown: (_view, event) => {
        // The mention list handles its keys in the capture phase and marks them
        // handled; swallow those so Enter neither sends nor splits the line.
        if (event.defaultPrevented) return true
        if (event.key === 'Enter' && !event.shiftKey && mentionOpen.current) return false
        if (event.key === 'Enter' && !event.shiftKey) {
          event.preventDefault()
          sendRef.current()
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor: ed }) => {
      onTyping?.()
      setEmpty(ed.isEmpty)
      const { from } = ed.state.selection
      const textBefore = ed.state.doc.textBetween(Math.max(0, from - 20), from, '\n')
      const mentionMatch = textBefore.match(/@(\w*)$/)
      setMentionQuery(mentionMatch ? mentionMatch[1] ?? '' : null)
    },
  })

  const mentionOpen = useRef(false)
  mentionOpen.current = mentionQuery !== null

  const handleSend = useCallback(() => {
    if (!editor || editor.isEmpty) return
    const content = editor.getHTML()
    const mentionMatches = content.match(/@(\w+)/g) || []
    onSend(content, mentionMatches.map((m) => m.slice(1)))
    editor.commands.clearContent()
    setEmpty(true)
  }, [editor, onSend])
  const sendRef = useRef(handleSend)
  sendRef.current = handleSend

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
      <div
        className={`grid gap-2 py-2.5 pr-2.5 pl-3.5 rounded-lg focus-within:field-focus
          ${tone === 'raised' ? 'bg-raised' : 'bg-base'}`}
      >
        <div className="min-w-0 max-h-50 overflow-y-auto py-1" onClick={() => editor?.commands.focus()}>
          <EditorContent editor={editor} />
        </div>
        <div className="flex items-center gap-0.5">
          {onFileUpload && (
            <>
              {/* eslint-disable-next-line no-restricted-syntax -- Ô chọn tệp luôn ẩn, chỉ mở hộp
                  thoại hệ thống qua fileRef.current.click(); không có mặt thị giác để primitive tạo hình. */}
              <input ref={fileRef} type="file" className="hidden" onChange={handleFileSelect} tabIndex={-1} />
              <IconButton
                onClick={() => fileRef.current?.click()}
                disabled={uploading}
                title="Đính kèm"
                aria-label="Đính kèm"
              >
                {uploading ? <Spinner size="sm" /> : <Paperclip size={18} strokeWidth={1.75} />}
              </IconButton>
            </>
          )}
          <IconButton
            onClick={() => setShowEmoji((v) => !v)}
            title="Biểu tượng cảm xúc"
            aria-label="Biểu tượng cảm xúc"
            aria-expanded={showEmoji}
          >
            <Smile size={18} strokeWidth={1.75} />
          </IconButton>
          {hint && <span className="ml-2 text-xs text-ink-muted truncate hidden sm:inline">{hint}</span>}
          <Button
            variant="primary"
            size="sm"
            onClick={handleSend}
            disabled={isPending || empty}
            className="ml-auto shrink-0"
          >
            <SendHorizontal size={16} strokeWidth={1.75} aria-hidden="true" />
            Gửi
          </Button>
        </div>
      </div>

      {showEmoji && (
        <div className="absolute bottom-full left-0 mb-2 z-dropdown">
          <EmojiPicker onSelect={handleEmojiSelect} onClose={() => setShowEmoji(false)} />
        </div>
      )}

      {mentionQuery !== null && (
        <div className="absolute bottom-full left-4 mb-1 z-dropdown">
          <MentionDropdown
            members={membersData?.members || []}
            people={people}
            query={mentionQuery}
            onSelect={(member) => {
              if (!editor) return
              const { from } = editor.state.selection
              const textBefore = editor.state.doc.textBetween(Math.max(0, from - 20), from, '\n')
              const match = textBefore.match(/@(\w*)$/)
              if (match) {
                const deleteFrom = from - match[0].length
                editor.chain()
                  .deleteRange({ from: deleteFrom, to: from })
                  .insertContent(`@${member.username} `)
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
