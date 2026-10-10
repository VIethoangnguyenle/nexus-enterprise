import { useRef, useState } from 'react'
import { PenLine, UserPlus, Users } from 'lucide-react'
import { MenuItem, Popover, Pressable } from '../primitives'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { StartChatDialog } from './StartChatDialog'

/**
 * "Trò chuyện mới": a menu with Nhắn tin trực tiếp and Tạo nhóm, and the two
 * dialogs behind it. The navigator column draws it on desktop; Trang chủ draws
 * it on phones, where that column is not on screen.
 */
export function NewChatMenu({ className, placement = 'bottom-start' }: { className: string; placement?: 'bottom-start' | 'bottom-end' }) {
  const triggerRef = useRef<HTMLButtonElement>(null)
  const [menuOpen, setMenuOpen] = useState(false)
  const [dialog, setDialog] = useState<'space' | 'dm' | null>(null)

  return (
    <>
      <Pressable
        ref={triggerRef}
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((o) => !o)}
        className={className}
      >
        <PenLine size={18} strokeWidth={1.75} className="text-accent" aria-hidden="true" />
        Trò chuyện mới
      </Pressable>
      <Popover
        open={menuOpen}
        onClose={() => setMenuOpen(false)}
        anchorRef={triggerRef}
        placement={placement}
        role="menu"
        label="Trò chuyện mới"
      >
        <MenuItem icon={<UserPlus size={16} strokeWidth={1.75} />} onClick={() => setDialog('dm')}>
          Nhắn tin trực tiếp
        </MenuItem>
        {/* Whether you may create spaces is only known server-side (create_channel on the
            workspace's channel area), so the item always shows and a 403 is explained. */}
        <MenuItem icon={<Users size={16} strokeWidth={1.75} />} onClick={() => setDialog('space')}>
          Tạo nhóm
        </MenuItem>
      </Popover>
      <CreateSpaceDialog open={dialog === 'space'} onClose={() => setDialog(null)} />
      <StartChatDialog open={dialog === 'dm'} onClose={() => setDialog(null)} />
    </>
  )
}
