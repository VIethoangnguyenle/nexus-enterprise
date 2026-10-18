import type { ChatSection } from '../../lib/chat-sections'
import { Pressable } from '../primitives'
import { NavSheet } from './NavSheet'

/**
 * Bottom sheet for picking which section a conversation belongs to
 * (phase 06, mockup §5 Mục row). Shows all the user's sections as radio
 * options, plus "Không có" to remove from any section.
 */
export function SectionPickerSheet({ sections, currentSectionId, onPick, onRemove, onClose }: {
  sections: ChatSection[]
  currentSectionId: string | undefined
  onPick: (sectionId: string) => void
  onRemove: () => void
  onClose: () => void
}) {
  return (
    <NavSheet label="Mục" onClose={onClose}>
      <div className="px-3.5 pt-1 pb-2">
        <h2 className="m-0 text-lg font-semibold text-ink">Mục</h2>
        <p className="m-0 text-small text-ink-muted">Chọn mục cho cuộc trò chuyện này</p>
      </div>
      <div role="radiogroup" aria-label="Mục" className="grid gap-1">
        {/* None option */}
        <Pressable
          role="radio"
          aria-checked={!currentSectionId}
          aria-label="Không có"
          onClick={() => {
            if (currentSectionId) onRemove()
            else onClose()
          }}
          className={`flex items-center gap-3.5 w-full min-h-16 px-3.5 rounded-lg text-left cursor-pointer border-none
            bg-transparent hover:bg-hover active:bg-hover transition-colors duration-quick focus-ring`}
        >
          <span className="grid gap-0.5 min-w-0 flex-1">
            <b className="text-16 font-semibold text-ink">Không có</b>
            <span className="text-small text-ink-muted">Về Tin nhắn trực tiếp hoặc Nhóm</span>
          </span>
          <RadioDot checked={!currentSectionId} />
        </Pressable>

        {/* Section options */}
        {sections.map((s) => {
          const checked = s.id === currentSectionId
          return (
            <Pressable
              key={s.id}
              role="radio"
              aria-checked={checked}
              onClick={() => {
                if (!checked) onPick(s.id)
                else onClose()
              }}
              className={`flex items-center gap-3.5 w-full min-h-16 px-3.5 rounded-lg text-left cursor-pointer border-none
                bg-transparent hover:bg-hover active:bg-hover transition-colors duration-quick focus-ring`}
            >
              <span className="grid gap-0.5 min-w-0 flex-1">
                <b className="text-16 font-semibold text-ink">{s.name}</b>
                <span className="text-small text-ink-muted">{s.channel_ids.length} cuộc trò chuyện</span>
              </span>
              <RadioDot checked={checked} />
            </Pressable>
          )
        })}
      </div>
    </NavSheet>
  )
}

function RadioDot({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={`grid place-items-center w-5.5 h-5.5 rounded-full border-2 shrink-0
        ${checked ? 'border-accent' : 'border-line'}`}
    >
      {checked && <span className="w-2.5 h-2.5 rounded-full bg-accent" />}
    </span>
  )
}
