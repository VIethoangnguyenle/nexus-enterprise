import { useAuthStore } from '../../stores/auth.store'
import { Avatar, IconButton } from '../primitives'
import { Search, Bell, HelpCircle, Settings } from 'lucide-react'

/** Nexus Enterprise global top bar — matches Stitch nexus-chat.html:
 *  h-16, bg-surface-container-lowest, border-b border-outline-variant/30, shadow-sm.
 *  Search: rounded-lg (not rounded-full), bg-surface-container-low, border-outline-variant/50.
 *  Action buttons: p-2 rounded-full, hover:bg-surface-container-high. */
export function TopBar() {
  const user = useAuthStore((s) => s.user)

  return (
    <header
      className="h-16 bg-surface-container-lowest border-b border-outline-variant/30
        flex items-center justify-between px-8 shrink-0 shadow-sm z-40"
    >
      {/* Brand — Stitch: font-h3 text-h3 */}
      <div className="flex items-center gap-4">
        <span className="text-h3 text-on-surface tracking-tight">Nexus Enterprise</span>
      </div>

      {/* Global search — Stitch: rounded-lg, bg-surface-container-low, border-outline-variant/50 */}
      <div className="hidden md:block flex-1 max-w-md mx-6">
        <div className="relative flex items-center w-full h-10 rounded-lg bg-surface-container-low
          border border-outline-variant/50 focus-within:border-primary focus-within:ring-2
          focus-within:ring-primary/10 transition-all">
          <Search
            size={20}
            className="ml-2 mr-1 text-on-surface-variant shrink-0"
          />
          {/* eslint-disable-next-line no-restricted-syntax -- Ô nhập trần trong hộp cha đã
              mang viền, nền và focus-within:ring. Cả hai variant của Input kèm thẻ bọc riêng
              cộng border + bg + padding + focus-ring, chồng lên thứ cha đã có, và thẻ bọc sẽ
              chiếm suất flex bên cạnh icon Search. Cùng dạng với ô tìm kiếm của ChatList —
              nếu mẫu này còn lặp, cân nhắc một composite SearchField ở mốc nghiệm thu chặng. */}
          <input
            type="text"
            placeholder="Search across Nexus..."
            className="w-full bg-transparent border-none focus:ring-0 text-small text-on-surface
              placeholder:text-on-surface-variant/70 h-full rounded-r-lg outline-none"
          />
        </div>
      </div>

      {/* Actions — Stitch: p-2 rounded-full, gap-space-sm */}
      <div className="flex items-center gap-2">
        <IconButton shape="round" size="lg" title="Notifications" aria-label="Notifications">
          <Bell size={20} />
        </IconButton>
        <IconButton shape="round" size="lg" className="hidden md:inline-flex" title="Help" aria-label="Help">
          <HelpCircle size={20} />
        </IconButton>
        <IconButton shape="round" size="lg" className="hidden md:inline-flex" title="Settings" aria-label="Settings">
          <Settings size={20} />
        </IconButton>

        {/* Avatar — Stitch: ml-space-sm w-8 h-8 rounded-full border border-outline-variant */}
        <div className="ml-2 w-8 h-8 rounded-full overflow-hidden border border-outline-variant cursor-pointer">
          <Avatar name={user?.username || 'U'} size="sm" />
        </div>
      </div>
    </header>
  )
}
