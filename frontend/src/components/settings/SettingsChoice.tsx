import { useRef, type KeyboardEvent, type ReactNode } from 'react'
import { Pressable, Select } from '../primitives'
import { SettingsRow, choiceLabel, type Choice } from './SettingsGroup'
import { useSettingsWide } from './settings-layout'

export interface SettingsChoiceProps<T extends string | number> {
  label: string
  hint?: ReactNode
  icon?: ReactNode
  value: T | undefined
  choices: Choice<T>[]
  /** Saves the picked value; the caller puts the old one back if the save fails. */
  onPick: (value: T) => void
  /** Phone: opens the page's choice subpage. */
  onOpen: () => void
  /** Set by an administrator: the line that says so. The row shows the value and offers no control. */
  locked?: string
  /** Phone: the value on the row when it reads shorter than the choice's own label. */
  valueLabel?: string
  /**
   * Wide: a drop-down even with fewer than four choices, for labels too long to sit side by side
   * (Mức mặc định cho nhóm mới in the mockup).
   */
  menu?: boolean
}

/**
 * A row that picks one value (DESIGN.md §6 "Chọn giá trị"). On a phone it is the row with the current value that
 * opens the page's choice subpage. Inside the desktop shell the choice is made in the row itself and saved at once:
 * a Segmented for two or three choices, a drop-down from four (§6 "Cài đặt trên máy tính").
 */
export function SettingsChoice<T extends string | number>({
  label, hint, icon, value, choices, onPick, onOpen, locked, valueLabel, menu,
}: SettingsChoiceProps<T>) {
  const wide = useSettingsWide()
  const shown = valueLabel ?? choiceLabel(choices, value)
  if (!wide) return <SettingsRow icon={icon} label={label} hint={hint} value={shown} locked={locked} onPress={onOpen} />
  if (locked) return <SettingsRow icon={icon} label={label} hint={hint} value={choiceLabel(choices, value)} locked={locked} />

  const pick = (next: T) => { if (next !== value) onPick(next) }
  const control = menu || choices.length >= 4
    ? (
      <div className="w-60 shrink-0">
        <Select
          aria-label={label}
          value={value == null ? '' : String(value)}
          onChange={(e) => {
            const next = choices.find((c) => String(c.value) === e.target.value)
            if (next) pick(next.value)
          }}
        >
          {value == null && <option value="" disabled />}
          {choices.map((c) => <option key={String(c.value)} value={String(c.value)}>{c.label}</option>)}
        </Select>
      </div>
    )
    : <Segmented label={label} value={value} choices={choices} onPick={pick} />
  return <SettingsRow icon={icon} label={label} hint={hint} trailing={control} />
}

/** Two or three exclusive choices side by side, with arrow-key movement (mockup settings-desktop.html `.seg`). */
export function Segmented<T extends string | number>({ label, value, choices, onPick }: {
  label: string
  value: T | undefined
  choices: Choice<T>[]
  onPick: (value: T) => void
}) {
  const refs = useRef(new Map<string, HTMLButtonElement | null>())
  const current = choices.findIndex((c) => c.value === value)
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    const target = choices[(Math.max(current, 0) + step + choices.length) % choices.length]
    if (!target) return
    e.preventDefault()
    onPick(target.value)
    refs.current.get(String(target.value))?.focus()
  }
  return (
    <div role="radiogroup" aria-label={label} onKeyDown={onKeyDown} className="inline-flex shrink-0 gap-0.5 p-0.75 rounded-lg bg-sunk">
      {choices.map((c, i) => {
        const on = c.value === value
        return (
          <Pressable
            key={String(c.value)}
            ref={(el) => { refs.current.set(String(c.value), el) }}
            role="radio"
            aria-checked={on}
            // One stop in the tab order: the chosen value, or the first when none is.
            tabIndex={on || (current < 0 && i === 0) ? 0 : -1}
            onClick={() => onPick(c.value)}
            className={`min-h-9 px-3 rounded-md text-[0.8125rem] font-semibold transition-colors duration-quick
              ${on ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'}`}
          >
            {c.label}
          </Pressable>
        )
      })}
    </div>
  )
}
