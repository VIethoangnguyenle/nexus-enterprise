import { useMemo, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { motion } from 'motion/react'
import { Check, ChevronDown, X } from 'lucide-react'
import {
  activeFilterCount, describeRange, isoDate, presetRange,
  type ChatSearchFilters, type DatePreset,
} from '../../lib/chat-search'
import type { Conversation } from '../../lib/conversations'
import { useMotionPresets } from '../../lib/motion'
import type { PeopleDirectory } from '../../lib/people'
import { matchesAll } from '../../lib/search-text'
import { Avatar, Button, FieldInput, IconButton, Popover, Pressable, ScrollFade, SearchField } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { NavSheet } from './NavSheet'

type PickerKind = 'from' | 'in' | 'date'

interface Props {
  filters: ChatSearchFilters
  /** Applies at once: there is no Tìm button. */
  onChange: (next: ChatSearchFilters) => void
  /** The conversations the person may read; the Trong list is made of them and nothing else. */
  conversations: Conversation[]
  /** The directory the person may browse; the Người gửi list is made of it and nothing else. */
  people: PeopleDirectory
  meId: string
  /** Chips scroll sideways and the lists open as bottom sheets. */
  phone: boolean
}

const PICKER_NAMES: Record<PickerKind, string> = { from: 'Người gửi', in: 'Trong', date: 'Ngày' }
const GROUP_SUB = (c: Conversation) => (c.kind === 'space' ? `Nhóm · ${c.memberCount} thành viên` : 'Trò chuyện trực tiếp')

/**
 * The filter chips of Chat's results (DESIGN.md "Gợi ý trong ô tìm", Hàng chip): Người gửi · Trong · Ngày open a
 * list, Có tệp · Có liên kết · Nhắc đến tôi switch. A chip with a value says the value (a name, a range of days)
 * and has a ✕ of its own; chips behind it shift by FLIP. The lists are Popovers on a desktop and bottom sheets on a
 * phone; they hold names and days, never an id, and choosing applies at once.
 */
export function ChatSearchFilterBar({ filters, onChange, conversations, people, meId, phone }: Props) {
  const m = useMotionPresets()
  const [open, setOpen] = useState<PickerKind | null>(null)
  const refs = {
    from: useRef<HTMLButtonElement>(null),
    in: useRef<HTMLButtonElement>(null),
    date: useRef<HTMLButtonElement>(null),
  }
  const today = useMemo(() => new Date(), [])
  const count = activeFilterCount(filters)

  const fromPerson = filters.from ? people.byUserId.get(filters.from) : undefined
  const channel = filters.channelId ? conversations.find((c) => c.id === filters.channelId) : undefined
  const range = filters.after || filters.beforeDate ? describeRange(filters, today) : ''

  // Chosen and cleared values: a phone brings the chip that just turned on into view.
  const apply = (next: ChatSearchFilters, chip?: RefObject<HTMLElement | null>) => {
    onChange(next)
    chip?.current?.scrollIntoView?.({ inline: 'nearest', block: 'nearest' })
  }
  const clear = (patch: Partial<ChatSearchFilters>, kind: PickerKind) => {
    onChange({ ...filters, ...patch })
    // The ✕ goes with the value; the focus stays on its chip.
    requestAnimationFrame(() => refs[kind].current?.focus())
  }
  const close = (kind: PickerKind) => {
    setOpen(null)
    refs[kind].current?.focus()
  }

  // ← and → walk the chips, as in any toolbar.
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const chips = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[data-chip]'))
    const at = chips.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    e.preventDefault()
    chips[(at + (e.key === 'ArrowRight' ? 1 : -1) + chips.length) % chips.length]?.focus()
  }

  const chipClass = phone ? 'h-9 text-sm' : 'h-8 text-[0.8125rem]'
  const picker = (
    kind: PickerKind,
    value: string,
    icon: ReactNode,
    onClear: () => void,
  ) => (
    <PickerChip
      key={kind}
      buttonRef={refs[kind]}
      name={PICKER_NAMES[kind]}
      value={value}
      icon={icon}
      open={open === kind}
      onOpen={() => setOpen(open === kind ? null : kind)}
      onClear={onClear}
      m={m}
      className={chipClass}
    />
  )
  const toggle = (name: string, on: boolean, flip: () => void) => (
    <ToggleChip key={name} name={name} on={on} onToggle={flip} m={m} className={chipClass} />
  )

  const chips = (
    <>
      {picker('from', fromPerson?.name ?? (filters.from ? 'Người đã rời' : ''),
        filters.from ? <Avatar name={fromPerson?.name ?? 'Người đã rời'} hueKey={filters.from} src={fromPerson?.avatarUrl} size={20} /> : null,
        () => clear({ from: undefined }, 'from'))}
      {picker('in', channel?.title ?? (filters.channelId ? 'Cuộc trò chuyện đã đóng' : ''),
        channel ? <ConversationIcon conversation={channel} size={20} /> : null,
        () => clear({ channelId: undefined }, 'in'))}
      {picker('date', range, null, () => clear({ after: undefined, beforeDate: undefined }, 'date'))}
      {toggle('Có tệp', !!filters.hasFile, () => apply({ ...filters, hasFile: !filters.hasFile || undefined }))}
      {toggle('Có liên kết', !!filters.hasLink, () => apply({ ...filters, hasLink: !filters.hasLink || undefined }))}
      {toggle('Nhắc đến tôi', !!filters.mentionsMe, () => apply({ ...filters, mentionsMe: !filters.mentionsMe || undefined }))}
      {count > 0 && (
        <Pressable
          onClick={() => onChange({})}
          className={`shrink-0 px-2 rounded-md font-semibold text-accent hover:bg-accent-wash transition-colors duration-quick
            ${phone ? 'h-9 text-sm' : 'h-8 text-[0.8125rem]'}`}
        >
          Xoá bộ lọc
        </Pressable>
      )}
    </>
  )

  const body = (kind: PickerKind) => (
    <PickerBody
      kind={kind}
      phone={phone}
      filters={filters}
      conversations={conversations}
      people={people}
      meId={meId}
      today={today}
      onPick={(next) => {
        apply(next, refs[kind])
        close(kind)
      }}
    />
  )

  return (
    <>
      {phone ? (
        <ScrollFade role="toolbar" aria-label="Bộ lọc" onKeyDown={onKeyDown} className="flex flex-nowrap items-center gap-2 px-3 pt-1.5 pb-2.5">
          {chips}
        </ScrollFade>
      ) : (
        <div role="toolbar" aria-label="Bộ lọc" onKeyDown={onKeyDown} className="flex flex-wrap items-center gap-2 px-5 pb-3">
          {chips}
        </div>
      )}

      {phone
        ? open && (
          <NavSheet label={PICKER_NAMES[open]} onClose={() => close(open)}>{body(open)}</NavSheet>
        )
        : (['from', 'in', 'date'] as const).map((kind) => (
          <Popover
            key={kind}
            open={open === kind}
            onClose={() => setOpen(null)}
            anchorRef={refs[kind]}
            role="dialog"
            label={PICKER_NAMES[kind]}
            className="w-80"
          >
            {body(kind)}
          </Popover>
        ))}
    </>
  )
}

/** The pill around a chip: the fill, and the accent wash with its hairline that fades in when the chip is on. */
function ChipShell({ on, m, className, children }: {
  on: boolean; m: ReturnType<typeof useMotionPresets>; className: string; children: ReactNode
}) {
  return (
    <motion.div
      layout={m.layoutProp}
      transition={m.layout}
      className={`relative inline-flex items-center shrink-0 rounded-full bg-sunk ${on ? '' : 'hover:bg-hover'} ${className}`}
    >
      <span
        aria-hidden="true"
        className={`absolute inset-0 rounded-full bg-accent-wash ring-1 ring-inset ring-accent pointer-events-none
          transition-opacity duration-quick ease-out motion-reduce:transition-none ${on ? 'opacity-100' : 'opacity-0'}`}
      />
      {children}
    </motion.div>
  )
}

function PickerChip({ buttonRef, name, value, icon, open, onOpen, onClear, m, className }: {
  buttonRef: RefObject<HTMLButtonElement | null>
  name: string
  value: string
  icon: ReactNode
  open: boolean
  onOpen: () => void
  onClear: () => void
  m: ReturnType<typeof useMotionPresets>
  className: string
}) {
  const on = value !== ''
  return (
    <ChipShell on={on} m={m} className={className}>
      <Pressable
        ref={buttonRef}
        data-chip=""
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={on ? `${name}: ${value}` : name}
        onClick={onOpen}
        className={`relative inline-flex items-center gap-1.5 h-full rounded-full font-semibold whitespace-nowrap
          ${on ? 'pl-2 pr-1.5' : 'px-3'} ${className.includes('h-9') ? 'pl-3.5' : ''}`}
      >
        {icon}
        <span className="max-w-40 truncate">{on ? value : name}</span>
        <ChevronDown
          size={16}
          strokeWidth={1.75}
          aria-hidden="true"
          className={`text-ink-muted transition-transform duration-quick ease-out motion-reduce:transition-none ${open ? 'rotate-180' : ''}`}
        />
      </Pressable>
      {on && (
        <motion.span
          initial={m.reduced ? { opacity: 0 } : { opacity: 0, scale: 0.6 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: m.reduced ? 0.12 : 0.16 }}
          className="relative mr-0.5 inline-flex"
        >
          <IconButton aria-label={`Bỏ lọc ${name}: ${value}`} title="Bỏ lọc" shape="round" size="md" onClick={onClear} className={className.includes('h-9') ? 'w-8 h-8' : 'w-7 h-7'}>
            <X size={14} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </motion.span>
      )}
    </ChipShell>
  )
}

function ToggleChip({ name, on, onToggle, m, className }: {
  name: string; on: boolean; onToggle: () => void; m: ReturnType<typeof useMotionPresets>; className: string
}) {
  return (
    <ChipShell on={on} m={m} className={className}>
      <Pressable
        data-chip=""
        aria-pressed={on}
        onClick={onToggle}
        className="relative inline-flex items-center gap-1.5 h-full px-3 rounded-full font-semibold whitespace-nowrap"
      >
        {on && (
          <motion.span
            initial={m.reduced ? { opacity: 0 } : { opacity: 0, scale: 0.6 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: m.reduced ? 0.12 : 0.16 }}
            className="inline-flex text-accent"
          >
            <Check size={16} strokeWidth={2} aria-hidden="true" />
          </motion.span>
        )}
        {name}
      </Pressable>
    </ChipShell>
  )
}

/** One row of a list: a radio, with ✓ on the current choice. */
function Pick({ checked, onPick, phone, children }: { checked: boolean; onPick: () => void; phone: boolean; children: ReactNode }) {
  return (
    <Pressable
      role="radio"
      aria-checked={checked}
      onClick={onPick}
      className={`flex items-center gap-2.5 w-full px-2.5 rounded-md text-sm hover:bg-hover transition-colors duration-quick
        ${phone ? 'min-h-14 gap-3' : 'min-h-11'}`}
    >
      {children}
      <Check size={16} strokeWidth={2} aria-hidden="true" className={`ml-auto shrink-0 text-accent ${checked ? '' : 'invisible'}`} />
    </Pressable>
  )
}

const Names = ({ title, sub }: { title: string; sub?: string }) => (
  <span className="min-w-0 grid text-left">
    <span className="truncate font-medium">{title}</span>
    {sub && <span className="truncate text-xs text-ink-muted">{sub}</span>}
  </span>
)

/** The inside of one list, the same for the popover and the sheet. */
function PickerBody({ kind, phone, filters, conversations, people, meId, today, onPick }: {
  kind: PickerKind
  phone: boolean
  filters: ChatSearchFilters
  conversations: Conversation[]
  people: PeopleDirectory
  meId: string
  today: Date
  onPick: (next: ChatSearchFilters) => void
}) {
  const [query, setQuery] = useState('')
  const words = query.trim()
  const avatar = phone ? 36 : 32
  const searchBox = (label: string) => (
    <SearchField
      label={label}
      tone="sunk"
      value={query}
      onChange={(e) => setQuery(e.target.value)}
      className={phone ? 'mx-3.5 mb-2 h-11 rounded-full [&_input]:text-16' : 'mx-0.5 mt-0.5 mb-1.5'}
    />
  )
  const list = 'grid gap-0.5 overflow-y-auto max-h-74'
  const title = (
    <h2 className={`m-0 font-display font-semibold ${phone ? 'px-3.5 pt-2 pb-2 text-[1.1875rem]' : 'sr-only'}`}>{PICKER_NAMES[kind]}</h2>
  )

  if (kind === 'from') {
    const me = people.byUserId.get(meId)
    const others = people.list.filter((p) => p.userId !== meId && (!words || matchesAll(p.name, words) || matchesAll(p.username, words)))
    const showMe = !!me && (!words || matchesAll(me.name, words) || matchesAll('bạn', words))
    return (
      <>
        {title}
        {searchBox('Tìm người')}
        <div role="radiogroup" aria-label="Người gửi" className={list}>
          {showMe && (
            <Pick checked={filters.from === meId} phone={phone} onPick={() => onPick({ ...filters, from: meId })}>
              <Avatar name={me.name} hueKey={meId} src={me.avatarUrl} size={avatar} />
              <Names title={`Bạn (${me.name})`} />
            </Pick>
          )}
          {others.map((p) => (
            <Pick key={p.userId} checked={filters.from === p.userId} phone={phone} onPick={() => onPick({ ...filters, from: p.userId })}>
              <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={avatar} />
              <Names title={p.name} sub={p.role || undefined} />
            </Pick>
          ))}
          {!showMe && others.length === 0 && <p className="m-0 px-3 py-4 text-small text-ink-muted">Không có ai tên “{words}”.</p>}
        </div>
      </>
    )
  }

  if (kind === 'in') {
    const found = conversations.filter((c) => !words || matchesAll(c.title, words))
    return (
      <>
        {title}
        {searchBox('Tìm cuộc trò chuyện')}
        <div role="radiogroup" aria-label="Trong" className={list}>
          {found.map((c) => (
            <Pick key={c.id} checked={filters.channelId === c.id} phone={phone} onPick={() => onPick({ ...filters, channelId: c.id })}>
              <ConversationIcon conversation={c} size={avatar} />
              <Names title={c.title} sub={GROUP_SUB(c)} />
            </Pick>
          ))}
          {found.length === 0 && <p className="m-0 px-3 py-4 text-small text-ink-muted">Không có cuộc trò chuyện nào tên “{words}”.</p>}
        </div>
      </>
    )
  }

  return <DatePick phone={phone} filters={filters} today={today} onPick={onPick} title={title} />
}

const PRESETS: { id: DatePreset; label: string }[] = [
  { id: 'today', label: 'Hôm nay' },
  { id: 'week', label: '7 ngày qua' },
  { id: 'month', label: '30 ngày qua' },
]

function DatePick({ phone, filters, today, onPick, title }: {
  phone: boolean; filters: ChatSearchFilters; today: Date; onPick: (next: ChatSearchFilters) => void; title: ReactNode
}) {
  const [custom, setCustom] = useState(false)
  const [from, setFrom] = useState(filters.after ?? '')
  const [to, setTo] = useState(filters.beforeDate ?? '')
  const current = describeRange(filters, today)
  const without: ChatSearchFilters = { ...filters, after: undefined, beforeDate: undefined }
  const preset = (id: DatePreset) => onPick({ ...without, ...presetRange(id, today) })
  const badRange = !!from && !!to && from > to
  const todayIso = isoDate(today)

  return (
    <>
      {title}
      <div role="radiogroup" aria-label="Ngày" className="grid gap-0.5">
        <Pick checked={!filters.after && !filters.beforeDate} phone={phone} onPick={() => onPick(without)}>
          <span className="font-medium">Bất kỳ lúc nào</span>
        </Pick>
        {PRESETS.map((p) => (
          <Pick key={p.id} checked={current === p.label} phone={phone} onPick={() => preset(p.id)}>
            <span className="font-medium">{p.label}</span>
          </Pick>
        ))}
        <div role="separator" className="h-px bg-line my-1 mx-1.5" />
        <Pick
          checked={!!current && !PRESETS.some((p) => p.label === current)}
          phone={phone}
          onPick={() => setCustom(true)}
        >
          <span className="font-medium">Tuỳ chọn khoảng…</span>
        </Pick>
      </div>
      {custom && (
        <div className="grid grid-cols-2 gap-2 px-2 pt-1.5 pb-1">
          <label className="grid gap-1 text-xs font-semibold text-ink-muted">
            Từ ngày
            <FieldInput type="date" value={from} max={to || todayIso} onChange={(e) => setFrom(e.target.value)} className={phone ? 'h-11 text-16' : ''} />
          </label>
          <label className="grid gap-1 text-xs font-semibold text-ink-muted">
            Đến ngày
            <FieldInput type="date" value={to} min={from || undefined} max={todayIso} onChange={(e) => setTo(e.target.value)} className={phone ? 'h-11 text-16' : ''} />
          </label>
          {badRange && <p role="alert" className="m-0 col-span-2 text-xs text-danger">Ngày bắt đầu phải trước ngày kết thúc.</p>}
          <Button
            size="sm"
            className={`col-span-2 ${phone ? 'h-11' : ''}`}
            disabled={(!from && !to) || badRange}
            onClick={() => onPick({ ...without, after: from || undefined, beforeDate: to || undefined })}
          >
            Áp dụng
          </Button>
        </div>
      )}
    </>
  )
}
