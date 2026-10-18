import { Children, useId, type ReactNode } from 'react'
import { Check, ChevronLeft, ChevronRight, Shield } from 'lucide-react'
import { Pressable } from '../primitives'

// The settings list of the phone (DESIGN.md §6 "Cài đặt", mockup mobile-shell.html §3, §4, §8, §11): groups on
// `--color-raised` radius 16, 16 from the edges, the title 13/600 muted outside; rows ≥ 52 with an icon box 32
// radius 9 on `--color-sunk`, label 15/500 and a 13px line, the current value muted on the right and a chevron.

// ---------------------------------------------------------------------------
// SettingsPage — a full-screen settings page: back, title, an optional action.
// ---------------------------------------------------------------------------

export interface SettingsPageProps {
  title: string
  onBack: () => void
  /** Name of the back button; "Quay lại" by default. */
  backLabel?: string
  /** At the right of the title: Lưu on a page that edits one field. */
  action?: ReactNode
  children: ReactNode
}

/** A settings page that fills the screen: the bar with back and title, then the groups scrolling under it. */
export function SettingsPage({ title, onBack, backLabel = 'Quay lại', action, children }: SettingsPageProps) {
  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label={title}>
      <header className="flex items-center gap-1 min-h-14 px-2 pt-safe">
        <Pressable
          aria-label={backLabel}
          onClick={() => onBack()}
          className="grid place-items-center w-11 h-11 rounded-full text-ink hover:bg-hover transition-colors duration-quick"
        >
          <ChevronLeft size={24} strokeWidth={1.75} aria-hidden="true" />
        </Pressable>
        <h1 className="flex-1 min-w-0 m-0 text-[1.0625rem] font-bold text-ink truncate">{title}</h1>
        {action && <div className="flex items-center shrink-0 pr-2">{action}</div>}
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto overscroll-contain">
        <div className="grid gap-5 max-w-2xl px-4 pt-2 pb-sheet">{children}</div>
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// SettingsGroup — a labelled card that holds rows.
// ---------------------------------------------------------------------------

export interface SettingsGroupProps {
  /** Title above the card, 13/600 muted. Omitted when the group has no label. */
  title?: string
  /** A line under the card, 13px muted. */
  note?: ReactNode
  children: ReactNode
  /** `radiogroup` for a page of exclusive choices. */
  role?: 'radiogroup'
  /** The group's name when it has no visible title. */
  label?: string
}

export function SettingsGroup({ title, note, children, role, label }: SettingsGroupProps) {
  const id = useId()
  return (
    <section aria-labelledby={title ? id : undefined} className="grid gap-1.5">
      {title && (
        <h2 id={id} className="m-0 px-1 text-[0.8125rem] font-semibold text-ink-muted">
          {title}
        </h2>
      )}
      {Children.toArray(children).length > 0 && (
        <div
          role={role}
          aria-labelledby={role && title ? id : undefined}
          aria-label={!title ? label : undefined}
          className="rounded-xl bg-raised divide-y divide-line overflow-hidden"
        >
          {children}
        </div>
      )}
      {note && <p className="m-0 px-1 text-[0.8125rem] text-ink-muted">{note}</p>}
    </section>
  )
}

// ---------------------------------------------------------------------------
// SettingsRow — one row: value + chevron, a custom trailing control, or locked.
// ---------------------------------------------------------------------------

export interface SettingsRowProps {
  /** Icon in a 32×32 box, radius 9, `--color-sunk`. */
  icon?: ReactNode
  /** Primary label, 15/500. */
  label: string
  /** Optional second line, 13px muted. */
  hint?: ReactNode
  /** Current value, muted on the right before the chevron. */
  value?: ReactNode
  /** Fires on press; the row then shows a chevron. */
  onPress?: () => void
  disabled?: boolean
  /**
   * Set by an administrator: the line that says so ("Quản trị viên NovaPay đã đặt"), shown under the label with a
   * shield. The row is dimmed to 72% and cannot be pressed; the server is what refuses a change.
   */
  locked?: string
  /** Danger styling (Đăng xuất, Rời workspace). */
  danger?: boolean
  /** Centre the label (Đăng xuất). */
  center?: boolean
  /** Instead of value and chevron: a switch, a check. */
  trailing?: ReactNode
  /** Extra ARIA for rows that are radios. */
  role?: 'radio'
  checked?: boolean
}

export function SettingsRow({
  icon, label, hint, value, onPress, disabled, locked, danger, center, trailing, role, checked,
}: SettingsRowProps) {
  const pressable = !!onPress && !disabled && !locked

  const content = (
    <>
      {icon && (
        <span aria-hidden="true" className="grid place-items-center w-8 h-8 rounded-surface bg-sunk text-ink-muted shrink-0">
          {icon}
        </span>
      )}
      <span className={`grid gap-0.5 min-w-0 flex-1 ${center ? 'justify-items-center' : ''}`}>
        <span className={`text-[0.9375rem] font-medium leading-snug truncate ${danger ? 'text-danger' : 'text-ink'}`}>{label}</span>
        {hint && <span className="text-[0.8125rem] leading-snug text-ink-muted">{hint}</span>}
        {locked && (
          <span className="flex items-center gap-1 text-[0.8125rem] leading-snug text-ink-muted">
            <Shield size={13} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
            {locked}
          </span>
        )}
      </span>
      {trailing ?? (
        <span className="flex items-center gap-1 min-w-0 max-w-[55%] shrink-0">
          {value != null && value !== '' && <span className="text-[0.9375rem] text-ink-muted truncate tnum">{value}</span>}
          {pressable && <ChevronRight size={18} strokeWidth={1.75} className="text-ink-subtle shrink-0" aria-hidden="true" />}
        </span>
      )}
    </>
  )

  const base = 'flex items-center gap-3 w-full min-h-13 px-4 py-2.5 text-left'
  if (pressable) {
    return (
      <Pressable
        role={role}
        aria-checked={role ? !!checked : undefined}
        onClick={onPress}
        className={`${base} hover:bg-hover active:bg-line transition-colors duration-quick`}
      >
        {content}
      </Pressable>
    )
  }
  return (
    <div className={`${base} ${locked ? 'opacity-72' : ''} ${disabled ? 'opacity-50' : ''}`} aria-disabled={disabled || !!locked || undefined}>
      {content}
    </div>
  )
}

// ---------------------------------------------------------------------------
// SettingsSwitch — a row whose trailing control is a 44×26 switch.
// ---------------------------------------------------------------------------

export interface SettingsSwitchProps {
  icon?: ReactNode
  label: string
  hint?: ReactNode
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  locked?: string
}

/** A switch saves the moment it is pressed (DESIGN.md §6 "Lưu"); the caller puts it back if the save fails. */
export function SettingsSwitch({ icon, label, hint, checked, onChange, disabled, locked }: SettingsSwitchProps) {
  const off = disabled || !!locked
  return (
    <SettingsRow
      icon={icon}
      label={label}
      hint={hint}
      locked={locked}
      disabled={disabled}
      trailing={
        <Pressable
          role="switch"
          aria-checked={checked}
          aria-label={label}
          disabled={off}
          onClick={() => onChange(!checked)}
          className={`relative shrink-0 w-11 h-6.5 rounded-full before:absolute before:-inset-2.5 before:content-['']
            ${off ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'}`}
        >
          <span
            aria-hidden="true"
            className={`absolute inset-0 rounded-full transition-colors duration-press ${checked ? 'bg-accent' : 'bg-sunk field-line'}`}
          />
          <span
            aria-hidden="true"
            className={`absolute top-0.5 left-0.5 w-5.5 h-5.5 rounded-full bg-raised shadow-overlay
              transition-transform duration-press motion-reduce:transition-none ${checked ? 'translate-x-4.5' : ''}`}
          />
        </Pressable>
      }
    />
  )
}

// ---------------------------------------------------------------------------
// ChoicePage — one column of radios: a tap picks and goes back.
// ---------------------------------------------------------------------------

export interface Choice<T extends string | number> {
  value: T
  label: string
  hint?: string
}

export interface ChoicePageProps<T extends string | number> {
  title: string
  value: T | undefined
  choices: Choice<T>[]
  onPick: (value: T) => void
  onBack: () => void
  /** Under the choices. */
  note?: ReactNode
}

/** Chọn giá trị (DESIGN.md §6): a page with one radio group; choosing saves (the caller's `onPick`) and goes back. */
export function ChoicePage<T extends string | number>({ title, value, choices, onPick, onBack, note }: ChoicePageProps<T>) {
  return (
    <SettingsPage title={title} onBack={onBack}>
      <SettingsGroup label={title} role="radiogroup" note={note}>
        {choices.map((c) => (
          <SettingsRow
            key={String(c.value)}
            role="radio"
            checked={c.value === value}
            label={c.label}
            hint={c.hint}
            onPress={() => { if (c.value !== value) onPick(c.value); onBack() }}
            trailing={
              <Check
                size={20}
                strokeWidth={2}
                aria-hidden="true"
                className={`shrink-0 text-accent ${c.value === value ? '' : 'invisible'}`}
              />
            }
          />
        ))}
      </SettingsGroup>
    </SettingsPage>
  )
}

/** The label of `value` among `choices`, for the row that opens them. */
export function choiceLabel<T extends string | number>(choices: Choice<T>[], value: T | undefined): string | undefined {
  return choices.find((c) => c.value === value)?.label
}
