import { useRef, useState } from 'react'
import { Spinner } from '../primitives'
import { PlusCircle, Send, Smile, AtSign } from 'lucide-react'

interface ChatInputProps {
  value: string
  onChange: (val: string) => void
  onSend: () => void
  onTyping?: () => void
  onFileUpload?: (file: File) => Promise<void>
  isPending?: boolean
  error?: string | null
  placeholder?: string
}

/** Plain textarea chat input matching Stitch nexus-chat.html:
 *  Container: border border-outline-variant/50 rounded-xl bg-surface-bright shadow-sm.
 *  Textarea: p-4 min-h-[80px] max-h-[200px].
 *  Bottom: p-2 pl-3 bg-surface-bright, rounded-full action buttons.
 *  Send: w-8 h-8 rounded-full bg-primary shadow-sm. */
export function ChatInput({ value, onChange, onSend, onTyping, onFileUpload, isPending, error, placeholder = 'Type a message...' }: ChatInputProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSend()
    }
  }

  const handleFileSelect = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file || !onFileUpload) return
    setUploading(true)
    try {
      await onFileUpload(file)
    } catch (err: any) {
      alert(`File upload failed: ${err.message}`)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className="p-4 md:p-6 border-t border-outline-variant/20 bg-surface-container-lowest z-10">
      {error && (
        <div className="bg-error-container text-on-error-container text-xs px-3 py-2 rounded-lg mb-2">{error}</div>
      )}

      {/* Input container — Stitch: border border-outline-variant/50 rounded-xl bg-surface-bright shadow-sm */}
      <div className="border border-outline-variant/50 rounded-xl bg-surface-bright
        focus-within:border-primary focus-within:ring-1 focus-within:ring-primary shadow-sm
        transition-all overflow-hidden flex flex-col">

        {/* Textarea — Stitch: p-4 min-h-[80px] max-h-[200px] */}
        <textarea
          value={value}
          onChange={e => { onChange(e.target.value); onTyping?.() }}
          onKeyDown={handleKeyDown}
          rows={2}
          placeholder={placeholder}
          className="w-full bg-transparent border-none text-sm text-on-surface resize-none
            focus:outline-none placeholder:text-on-surface-variant/50 min-h-[80px] max-h-[200px]
            font-[inherit] leading-relaxed p-4"
        />

        {/* Bottom actions — Stitch: flex items-center justify-between p-2 pl-3 bg-surface-bright */}
        <div className="flex items-center justify-between p-2 pl-3 bg-surface-bright">
          {/* Left: action buttons — Stitch: p-1.5 rounded-full */}
          <div className="flex items-center gap-1">
            {onFileUpload && (
              <>
                <input ref={fileRef} type="file" className="hidden" onChange={handleFileSelect} />
                <button
                  onClick={() => fileRef.current?.click()}
                  disabled={uploading}
                  title="Add file"
                  className="p-1.5 rounded-full text-on-surface-variant hover:bg-surface-container-high
                    transition-colors border-none bg-transparent cursor-pointer
                    disabled:opacity-30 disabled:cursor-not-allowed"
                >
                  {uploading ? <Spinner size="sm" /> : <PlusCircle size={22} />}
                </button>
              </>
            )}
            <button
              title="Emoji"
              className="p-1.5 rounded-full text-on-surface-variant hover:bg-surface-container-high
                transition-colors border-none bg-transparent cursor-pointer"
            >
              <Smile size={22} />
            </button>
            <button
              title="Mention"
              className="p-1.5 rounded-full text-on-surface-variant hover:bg-surface-container-high
                transition-colors border-none bg-transparent cursor-pointer"
            >
              <AtSign size={22} />
            </button>
          </div>

          {/* Right: hint + send — Stitch: w-8 h-8 rounded-full bg-primary shadow-sm */}
          <div className="flex items-center gap-3 pr-2">
            <span className="text-xs text-on-surface-variant hidden md:inline">Press Enter to send</span>
            <button
              onClick={onSend}
              disabled={!value.trim() || isPending}
              className="w-8 h-8 flex items-center justify-center rounded-full
                bg-primary text-on-primary border-none cursor-pointer transition-colors
                hover:bg-primary/90 shadow-sm
                disabled:opacity-30 disabled:cursor-not-allowed flex-shrink-0"
            >
              {isPending ? <Spinner size="sm" /> : <Send size={18} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
