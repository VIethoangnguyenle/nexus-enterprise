import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { useRef, useState, useCallback } from 'react'
import { EmojiPicker } from './EmojiPicker'
import { MentionDropdown } from './MentionDropdown'
import { EditorToolbar } from './EditorToolbar'
import { Spinner, IconButton } from '../primitives'
import { useChannelMembers } from '../../hooks/useMessaging'
import { PlusCircle, Smile, AtSign, Paperclip, Send } from 'lucide-react'

interface ChatEditorProps {
  onSend: (content: string, mentions: string[]) => void
  onTyping?: () => void
  onFileUpload?: (file: File) => Promise<void>
  isPending?: boolean
  error?: string | null
  placeholder?: string
  channelId: string
}

/** Rich text editor matching Stitch nexus-chat.html:
 *  Outer: p-gutter (24px) border-t border-outline-variant/20 bg-surface-container-lowest.
 *  Container: border border-outline-variant/50 rounded-xl bg-surface-bright
 *    focus-within:border-primary focus-within:ring-1 shadow-sm.
 *  Toolbar: p-2 border-b border-outline-variant/20 bg-surface-container-low.
 *  Textarea: p-4 min-h-[80px] max-h-[200px].
 *  Bottom: p-2 pl-3 bg-surface-bright, left = rounded-full action buttons, right = hint + send. */
export function ChatEditor({
  onSend,
  onTyping,
  onFileUpload,
  isPending,
  error,
  placeholder = 'Type a message...',
  channelId,
}: ChatEditorProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [showEmoji, setShowEmoji] = useState(false)
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const { data: membersData } = useChannelMembers(channelId)
  const [uploadError, setUploadError] = useState<string | null>(null)

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
        codeBlock: { HTMLAttributes: { class: 'chat-code-block' } },
      }),
      Placeholder.configure({ placeholder }),
    ],
    editorProps: {
      attributes: {
        class: 'chat-editor-content',
      },
      handleKeyDown: (_view, event) => {
        if (event.key === 'Enter' && !event.shiftKey) {
          event.preventDefault()
          handleSend()
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor: ed }) => {
      onTyping?.()
      // Detect @mention: check text before cursor for @ pattern
      const { from } = ed.state.selection
      const textBefore = ed.state.doc.textBetween(Math.max(0, from - 20), from, '\n')
      const mentionMatch = textBefore.match(/@(\w*)$/)
      if (mentionMatch) {
        setMentionQuery(mentionMatch[1])
      } else {
        setMentionQuery(null)
      }
    },
  })

  const handleSend = useCallback(() => {
    if (!editor || editor.isEmpty) return
    const content = editor.getHTML()
    const mentionMatches = content.match(/@(\w+)/g) || []
    const mentions = mentionMatches.map(m => m.slice(1))
    onSend(content, mentions)
    editor.commands.clearContent()
  }, [editor, onSend])

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
    } catch (err: any) {
      setUploadError(err.message || 'Upload failed')
      setTimeout(() => setUploadError(null), 5000)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    /* Stitch: p-gutter border-t border-outline-variant/20 bg-surface-container-lowest z-10 */
    <div className="p-4 md:p-6 border-t border-outline-variant/20 bg-surface-container-lowest z-10 relative">
      {(error || uploadError) && (
        <div className="bg-error-container text-on-error-container text-xs px-3 py-2 rounded-lg mb-2">
          {error || uploadError}
        </div>
      )}

      {/* Input container — Stitch: border border-outline-variant/50 rounded-xl bg-surface-bright
          focus-within:border-primary focus-within:ring-1 shadow-sm overflow-hidden flex flex-col */}
      <div className="border border-outline-variant/50 rounded-xl bg-surface-bright
        focus-within:border-primary focus-within:ring-1 focus-within:ring-primary shadow-sm
        transition-all overflow-hidden flex flex-col relative">

        {/* Toolbar — Stitch: p-2 border-b border-outline-variant/20 bg-surface-container-low */}
        <div className="flex items-center gap-1 p-2 border-b border-outline-variant/20 bg-surface-container-low">
          {editor && <EditorToolbar editor={editor} />}
        </div>

        {/* TipTap Editor — Stitch: p-4 min-h-[80px] max-h-[200px] */}
        <div className="min-w-0 min-h-[80px] max-h-[200px] overflow-y-auto p-4">
          <EditorContent editor={editor} />
        </div>

        {/* Bottom actions — Stitch: flex items-center justify-between p-2 pl-3 bg-surface-bright */}
        <div className="flex items-center justify-between p-2 pl-3 bg-surface-bright">
          {/* Left: attachment icons — Stitch: p-1.5 rounded-full */}
          <div className="flex items-center gap-1">
            {/* File upload */}
            {onFileUpload && (
              <>
                <input ref={fileRef} type="file" className="hidden" onChange={handleFileSelect} />
                <IconButton
                  onClick={() => fileRef.current?.click()}
                  disabled={uploading}
                  title="Add file"
                  aria-label="Add file"
                  className="rounded-full disabled:opacity-30 disabled:cursor-not-allowed"
                >
                  {uploading ? <Spinner size="sm" /> : <PlusCircle size={22} />}
                </IconButton>
              </>
            )}
            {/* Emoji */}
            <IconButton
              onClick={() => setShowEmoji(!showEmoji)}
              title="Emoji"
              aria-label="Emoji"
              className="rounded-full"
            >
              <Smile size={22} />
            </IconButton>
            {/* Mention — inserts @ to trigger autocomplete */}
            <IconButton
              onClick={() => {
                editor?.commands.insertContent('@')
                editor?.commands.focus()
              }}
              title="Mention someone"
              aria-label="Mention someone"
              className="rounded-full"
            >
              <AtSign size={22} />
            </IconButton>
          </div>

          {/* Right: hint + send — Stitch: w-8 h-8 rounded-full bg-primary shadow-sm */}
          <div className="flex items-center gap-3 pr-2">
            <span className="text-xs text-on-surface-variant hidden md:inline">Press Enter to send</span>
            <IconButton
              onClick={handleSend}
              disabled={isPending || (editor?.isEmpty ?? true)}
              aria-label="Send message"
              className="bg-primary text-on-primary hover:bg-primary/90 rounded-full shadow-sm
                disabled:opacity-30 disabled:cursor-not-allowed flex-shrink-0"
            >
              {isPending ? <Spinner size="sm" /> : <Send size={18} />}
            </IconButton>
          </div>
        </div>

        {/* @mention and emoji picker moved outside overflow-hidden container */}
      </div>

      {/* Emoji picker popup — OUTSIDE overflow-hidden container for visibility */}
      {showEmoji && (
        <div className="absolute bottom-full left-0 mb-2 z-50">
          <EmojiPicker onSelect={handleEmojiSelect} onClose={() => setShowEmoji(false)} />
        </div>
      )}

      {/* @mention autocomplete dropdown — positioned outside overflow-hidden container */}
      {mentionQuery !== null && (
        <div className="absolute bottom-full left-4 mb-1 z-50">
          <MentionDropdown
            members={membersData?.members || []}
            query={mentionQuery}
            onSelect={(member) => {
              if (!editor) return
              // Delete the @query text and insert @username
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
