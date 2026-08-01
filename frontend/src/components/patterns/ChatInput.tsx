import { useRef, useState } from 'react'
import { IconButton, Spinner } from '../primitives'
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
 *  Textarea: p-4 min-h-20 max-h-50 (80px/200px on the 4px scale).
 *  Bottom: p-2 pl-3 bg-surface-bright, rounded-full action buttons.
 *  Send: w-8 h-8 rounded-full bg-primary shadow-sm — IconButton variant="filled" size="md". */
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

        {/* Textarea — Stitch: p-4 min-h-20 max-h-50 (80px/200px trên thang 4px, xác nhận qua
            dist CSS: max-w-60{max-width:calc(var(--spacing) * 60)}, --spacing: .25rem) */}
        {/* eslint-disable-next-line no-restricted-syntax -- Textarea primitive khoá cứng
            bg-surface-container-lowest, border border-outline-variant, rounded-md, resize-y,
            min-h-18, px-3 py-2 và tự có focus-ring. Cả sáu chọi thẳng với bg-transparent/
            border-none/(không bo góc)/resize-none/p-4/min-h-20 mà ô này cần: nó lồng trong khung
            ngoài đã tự có border rounded-xl riêng (dòng dưới), không phải khung của chính nó, và
            dải focus dùng chung với khung ngoài qua focus-within — tự thêm focus-ring sẽ nhân đôi
            vòng focus. Ép vào sẽ tạo khung viền lồng khung viền, cùng lớp lỗi độ đặc hiệu đã sửa
            bốn lần trong đợt di cư này (xem bảng ở đầu kế hoạch). */}
        <textarea
          value={value}
          onChange={e => { onChange(e.target.value); onTyping?.() }}
          onKeyDown={handleKeyDown}
          rows={2}
          placeholder={placeholder}
          className="w-full bg-transparent border-none text-sm text-on-surface resize-none
            focus:outline-none placeholder:text-on-surface-variant/50 min-h-20 max-h-50
            font-[inherit] leading-relaxed p-4"
        />

        {/* Bottom actions — Stitch: flex items-center justify-between p-2 pl-3 bg-surface-bright */}
        <div className="flex items-center justify-between p-2 pl-3 bg-surface-bright">
          {/* Left: action buttons — Stitch: p-1.5 rounded-full */}
          <div className="flex items-center gap-1">
            {onFileUpload && (
              <>
                {/* eslint-disable-next-line no-restricted-syntax -- Không primitive nào mô phỏng
                    input type=file: Input bọc thêm div flex-col và khoá cứng border/padding/bg
                    cho ô nhập văn bản hiển thị — vô nghĩa với input ẩn (className="hidden") chỉ
                    để kích hoạt bằng lập trình qua fileRef. ChatEditor.tsx có input giống hệt,
                    cũng chưa migrate (2 warning còn treo, xem npx eslint trên file đó), xác nhận
                    đây là khoảng trống chưa có trong bộ primitive chứ không phải bị bỏ sót. */}
                <input ref={fileRef} type="file" className="hidden" onChange={handleFileSelect} />
                {/* eslint-disable-next-line no-restricted-syntax -- Đo trực tiếp trên
                    ChatEditor.tsx (đang chạy live, cùng dùng IconButton className="rounded-full"
                    cho ba nút hàng trái này): getComputedStyle trả về border-radius 8px, không
                    tròn — variant `ghost` khoá cứng rounded-md, className="rounded-full" truyền
                    vào thua theo thứ tự stylesheet, cùng lớp lỗi độ đặc hiệu đã sửa ở
                    Button/Input/NavRow/chính IconButton trước đó. Stitch quy định hàng nút này
                    p-1.5 rounded-full (tròn) nhưng `ghost` không có hình tròn để chọn — ép vào sẽ
                    lặng lẽ đổi từ tròn sang bo góc 8px. Cỡ hộp cũng lệch: p-1.5 quanh icon 22px ra
                    hộp ~34px, không khớp sm/md/lg (28/32/36px) của IconButton. */}
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
            {/* eslint-disable-next-line no-restricted-syntax -- Cùng lý do với nút Add file phía
                trên: IconButton `ghost` khoá cứng rounded-md, đo được thắng className="rounded-full"
                trên ChatEditor.tsx live (border-radius 8px thay vì tròn). */}
            <button
              title="Emoji"
              className="p-1.5 rounded-full text-on-surface-variant hover:bg-surface-container-high
                transition-colors border-none bg-transparent cursor-pointer"
            >
              <Smile size={22} />
            </button>
            {/* eslint-disable-next-line no-restricted-syntax -- Cùng lý do với nút Add file phía
                trên: IconButton `ghost` khoá cứng rounded-md, đo được thắng className="rounded-full"
                trên ChatEditor.tsx live (border-radius 8px thay vì tròn). */}
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
            {/* IconButton variant="filled" size="md" = w-8 h-8 rounded-full bg-primary
                shadow-sm — khớp Stitch gần như nguyên văn (xem comment đầu file). `filled` tự
                khai trọn bg/text/bo góc/shadow trong chuỗi biến thể nên không có gì để className
                chọi vào. Hai khác biệt hình ảnh được chấp nhận so với bản thô cũ:
                hover:bg-primary/90 (primary pha trong suốt) -> hover:bg-primary-hover (#003EA8
                đặc, token chuẩn hoá của `filled`), và disabled:opacity-30 -> opacity-40 (khoá
                cứng trong base của IconButton, dùng chung mọi variant). */}
            <IconButton
              variant="filled"
              size="md"
              onClick={onSend}
              disabled={!value.trim() || isPending}
              aria-label="Send message"
              className="flex-shrink-0"
            >
              {isPending ? <Spinner size="sm" /> : <Send size={18} />}
            </IconButton>
          </div>
        </div>
      </div>
    </div>
  )
}
