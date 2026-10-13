import { useRef, useState } from 'react'
import { FolderPlus, Plus, UserPlus, Users } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatSections } from '../../hooks/useChatSections'
import { Pressable } from '../primitives'
import { PhoneSheetRow } from '../patterns/PhoneSheetRow'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { NavSheet } from './NavSheet'
import { useSectionOverlays } from './SectionOverlays'
import { StartChatDialog } from './StartChatDialog'

type Step = 'sheet' | 'dm' | 'space' | null

const ICON = { size: 20, strokeWidth: 1.75 } as const

/**
 * The + beside the Chat bottom bar (DESIGN.md §5, Điều hướng di động): 64×64,
 * and a sheet "Trò chuyện mới" with Nhắn tin trực tiếp, Tạo nhóm and Tạo mục,
 * each opening the dialog that already does the job. "Duyệt nhóm" joins them
 * when listing open groups has a server to ask.
 */
export function NewChatButton() {
  const { workspaceId } = useActiveWorkspace()
  const sections = useChatSections(workspaceId)
  const overlays = useSectionOverlays(sections, true)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const [step, setStep] = useState<Step>(null)

  const backToButton = () => {
    setStep(null)
    triggerRef.current?.focus()
  }

  return (
    <>
      <Pressable
        ref={triggerRef}
        onClick={() => setStep('sheet')}
        aria-haspopup="dialog"
        aria-label="Trò chuyện mới"
        className="fixed right-3.5 bottom-[calc(1.375rem+env(safe-area-inset-bottom,0px))] z-sticky grid place-items-center w-16 h-16
          rounded-[1.25rem] bg-accent-wash text-accent shadow-overlay transition-transform duration-press active:scale-95
          motion-reduce:transition-none lg:hidden"
      >
        <Plus size={28} strokeWidth={2} aria-hidden="true" />
      </Pressable>

      {step === 'sheet' && (
        <NavSheet label="Trò chuyện mới" onClose={backToButton}>
          <h2 className="m-0 px-3.5 pt-1 pb-2 font-display text-[1.0625rem] font-bold text-ink">Trò chuyện mới</h2>
          <PhoneSheetRow icon={<UserPlus {...ICON} />} hint="Chọn một người" onClick={() => setStep('dm')}>
            Nhắn tin trực tiếp
          </PhoneSheetRow>
          {/* Whether you may create groups is only known server-side, so the row always shows and a 403 is explained. */}
          <PhoneSheetRow icon={<Users {...ICON} />} hint="Nơi cả nhóm trò chuyện theo chủ đề" onClick={() => setStep('space')}>
            Tạo nhóm
          </PhoneSheetRow>
          <PhoneSheetRow
            icon={<FolderPlus {...ICON} />}
            hint="Gom các cuộc trò chuyện của bạn"
            onClick={() => {
              setStep(null)
              overlays.openCreate(triggerRef.current, undefined, 'bottom-end')
            }}
          >
            Tạo mục
          </PhoneSheetRow>
        </NavSheet>
      )}
      <StartChatDialog open={step === 'dm'} onClose={backToButton} />
      <CreateSpaceDialog open={step === 'space'} onClose={backToButton} />
      {overlays.node}
    </>
  )
}
