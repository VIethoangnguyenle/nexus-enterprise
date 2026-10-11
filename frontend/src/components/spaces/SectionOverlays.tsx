import { useRef, useState, type ComponentType, type ReactNode } from 'react'
import { ArrowDown, ArrowUp, Bell, BellOff, Check, Folder, FolderPlus, Pencil, Star, Trash2, X } from 'lucide-react'
import type { ChatSectionsApi, ConversationRef } from '../../hooks/useChatSections'
import { useStarConversation } from '../../hooks/useShortcuts'
import { changeNotificationLevel } from '../../hooks/useNotificationLevel'
import { explain, statusOf } from '../../lib/errors'
import { MAX_SECTION_NAME, sectionOfChannel, validateSectionName, type ChatSection } from '../../lib/chat-sections'
import { Button, MenuItem, MenuSeparator, Popover, TextField } from '../primitives'
import { NavSheet, SheetItem, SheetLabel, SheetSeparator } from './NavSheet'

type Placement = 'bottom-start' | 'bottom-end' | 'top-start'

/** A conversation the row menu opens for: its name for the toasts, and the state the menu changes. */
export interface RowTarget extends ConversationRef {
  starred?: boolean
  /** Notification level is `off`. */
  muted?: boolean
}

type Overlay =
  | { kind: 'row'; conversation: RowTarget }
  | { kind: 'section'; sectionId: string }
  | { kind: 'create'; conversation?: ConversationRef; placement: Placement }
  | { kind: 'rename'; sectionId: string }

/**
 * One row of a menu, drawn as a popover item on a desktop and as a 52px sheet
 * row on a phone. `checked` makes it a radio row (the section a conversation
 * is in now).
 */
interface ItemProps {
  icon?: ReactNode
  onClick: () => void
  tone?: 'default' | 'danger'
  checked?: boolean
  disabled?: boolean
  children: ReactNode
}
interface Kit {
  Item: ComponentType<ItemProps>
  Separator: ComponentType
  /** A named group of radio rows. */
  Radios: ComponentType<{ label: string; children: ReactNode }>
}

const popoverKit: Kit = {
  Item: ({ icon, onClick, tone, checked, disabled, children }) => (
    <MenuItem
      icon={icon}
      tone={tone}
      disabled={disabled}
      onClick={onClick}
      {...(checked !== undefined ? { role: 'menuitemradio', 'aria-checked': checked } : {})}
    >
      <span className="flex-1 min-w-0 truncate">{children}</span>
      {checked && <Check size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent shrink-0" />}
    </MenuItem>
  ),
  Separator: MenuSeparator,
  Radios: ({ label, children }) => (
    <>
      <div className="px-2.5 pt-1 pb-1 text-xs font-semibold text-ink-muted">{label}</div>
      <div className="grid gap-0.5">{children}</div>
    </>
  ),
}

const sheetKit: Kit = {
  Item: ({ icon, onClick, tone, checked, disabled, children }) => (
    <SheetItem icon={icon} onClick={onClick} tone={tone} checked={checked} disabled={disabled}>{children}</SheetItem>
  ),
  Separator: SheetSeparator,
  Radios: ({ label, children }) => (
    <>
      <SheetLabel>{label}</SheetLabel>
      <div role="radiogroup" aria-label={label} className="grid gap-1">{children}</div>
    </>
  ),
}

/** The id of the system section, for its folded state. */
export const STARRED_SECTION = 'starred'

const ICON = { size: 16, strokeWidth: 1.75 } as const
const ICON_SHEET = { size: 18, strokeWidth: 1.75 } as const

/**
 * Everything that opens from the navigator's section controls: the menu of a
 * conversation ("Chuyển tới mục", Gắn sao, Tắt thông báo), the menu of a section, and the name form
 * ("Tên mục mới" / "Đổi tên mục"). On a desktop they are popovers hung from the
 * control that opened them; on a phone they are bottom sheets
 * (design/mockups/spaces.html §1, chat-mobile.html).
 *
 * Render `node` once in the view. `openRowMenu`, `openSectionMenu` and
 * `openCreate` take the element to anchor to (and to hand focus back to).
 * `onReveal` is told the section a conversation was just moved into, so the
 * view can open it if it was folded.
 */
export function useSectionOverlays(api: ChatSectionsApi, phone: boolean, onReveal?: (sectionId: string) => void) {
  const [open, setOpen] = useState<Overlay | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const anchor = useRef<HTMLElement | null>(null)

  const show = (overlay: Overlay, el: HTMLElement | null) => {
    anchor.current = el
    setOpen(overlay)
  }
  const close = () => setOpen(null)
  // A menu item that opens another overlay replaces `open` before the menu's own
  // close runs; closing only if still current keeps the new one.
  const closeIfCurrent = (overlay: Overlay) => () => setOpen((cur) => (cur === overlay ? null : cur))

  const star = useStarConversation()
  const { Item, Separator, Radios } = phone ? sheetKit : popoverKit
  const icon = phone ? ICON_SHEET : ICON
  const sections = api.sections

  const renderMenu = (overlay: Overlay, label: string, body: ReactNode) =>
    phone ? (
      <NavSheet key={overlay.kind} label={label} onClose={closeIfCurrent(overlay)}>{body}</NavSheet>
    ) : (
      <Popover key={overlay.kind} open onClose={closeIfCurrent(overlay)} anchorRef={anchor} role="menu" label={label}>{body}</Popover>
    )

  let node: ReactNode = null

  if (open?.kind === 'row') {
    const c = open.conversation
    const current = sectionOfChannel(sections, c.id)
    // A starred conversation is listed under Đã gắn sao only: it keeps its section on the server, and moving it waits for the unstar.
    const arrange = !c.starred
    node = renderMenu(
      open,
      `Tuỳ chọn cho ${c.title}`,
      <>
        {arrange && sections.length > 0 && (
          <Radios label="Chuyển tới mục">
            {sections.map((s) => (
              <Item
                key={s.id}
                icon={<Folder {...icon} />}
                checked={s.id === current?.id}
                onClick={() => {
                  if (phone) close()
                  if (s.id === current?.id) return
                  onReveal?.(s.id)
                  void api.placeConversation(c, s.id)
                }}
              >
                {s.name}
              </Item>
            ))}
          </Radios>
        )}
        {arrange && (
          <Item
            icon={<FolderPlus {...icon} />}
            onClick={() => show({ kind: 'create', conversation: c, placement: 'bottom-start' }, anchor.current)}
          >
            Mục mới…
          </Item>
        )}
        {arrange && current && (
          <Item
            icon={<X {...icon} />}
            onClick={() => {
              if (phone) close()
              void api.unplaceConversation(c)
            }}
          >
            Bỏ khỏi mục
          </Item>
        )}
        {arrange && <Separator />}
        <Item
          icon={<Star {...icon} />}
          onClick={() => {
            close()
            if (!c.starred) onReveal?.(STARRED_SECTION)
            void star({ channelId: c.id, name: c.title, starred: !c.starred })
          }}
        >
          {c.starred ? 'Bỏ gắn sao' : 'Gắn sao'}
        </Item>
        <Item
          icon={c.muted ? <Bell {...icon} /> : <BellOff {...icon} />}
          onClick={() => {
            close()
            void changeNotificationLevel({ channelId: c.id, name: c.title, level: c.muted ? 'main' : 'off' })
          }}
        >
          {c.muted ? 'Bật thông báo' : 'Tắt thông báo'}
        </Item>
      </>,
    )
  } else if (open?.kind === 'section') {
    const i = sections.findIndex((s) => s.id === open.sectionId)
    const section = sections[i]
    if (section) {
      const move = (dir: 'up' | 'down') => {
        if (phone) close()
        void api.moveSection(section.id, dir).then(() => setAnnouncement(
          `Đã chuyển mục “${section.name}” ${dir === 'up' ? 'lên' : 'xuống'}`,
        ))
      }
      node = renderMenu(
        open,
        `Tuỳ chọn cho mục ${section.name}`,
        <>
          <Item
            icon={<Pencil {...icon} />}
            onClick={() => show({ kind: 'rename', sectionId: section.id }, anchor.current)}
          >
            Đổi tên mục
          </Item>
          <Item icon={<ArrowUp {...icon} />} disabled={i === 0} onClick={() => move('up')}>Chuyển lên</Item>
          <Item icon={<ArrowDown {...icon} />} disabled={i === sections.length - 1} onClick={() => move('down')}>
            Chuyển xuống
          </Item>
          <Separator />
          <Item
            icon={<Trash2 {...icon} />}
            tone="danger"
            onClick={() => {
              if (phone) close()
              void api.deleteSection(section.id)
            }}
          >
            Xoá mục
          </Item>
        </>,
      )
    }
  } else if (open?.kind === 'create' || open?.kind === 'rename') {
    const renaming = open.kind === 'rename' ? sections.find((s) => s.id === open.sectionId) : undefined
    if (open.kind === 'create' || renaming) {
      const label = renaming ? 'Đổi tên mục' : 'Tên mục mới'
      const conversation = open.kind === 'create' ? open.conversation : undefined
      const form = (
        <SectionNameForm
          key={renaming?.id ?? 'new'}
          label={label}
          submitLabel={renaming ? 'Lưu' : 'Tạo'}
          initial={renaming?.name ?? ''}
          large={phone}
          sections={sections}
          exceptId={renaming?.id}
          onCancel={close}
          onSubmit={async (name) => {
            if (renaming) {
              if (name.trim() !== renaming.name) await api.renameSection(renaming.id, name)
            } else {
              await api.createSection(name, conversation)
            }
            close()
            anchor.current?.focus()
          }}
        />
      )
      node = phone ? (
        <NavSheet key="name" label={label} onClose={close}>{form}</NavSheet>
      ) : (
        <Popover
          key="name"
          open
          onClose={close}
          anchorRef={anchor}
          role="dialog"
          label={label}
          placement={open.kind === 'create' ? open.placement : 'bottom-start'}
          className="w-72"
        >
          {form}
        </Popover>
      )
    }
  }

  return {
    openRowMenu: (conversation: RowTarget, el: HTMLElement | null) => show({ kind: 'row', conversation }, el),
    openSectionMenu: (sectionId: string, el: HTMLElement | null) => show({ kind: 'section', sectionId }, el),
    openCreate: (el: HTMLElement | null, conversation?: ConversationRef, placement: Placement = 'bottom-start') =>
      show({ kind: 'create', conversation, placement }, el),
    node: (
      <>
        {node}
        <div role="status" className="sr-only">{announcement}</div>
      </>
    ),
  }
}

/** What `useSectionOverlays` hands the view. */
export type SectionOverlays = ReturnType<typeof useSectionOverlays>

/** "Tên mục" form: Enter confirms, Esc closes (the popover's own), errors are said in words. */
function SectionNameForm({ label, submitLabel, initial, large, sections, exceptId, onCancel, onSubmit }: {
  label: string
  submitLabel: string
  initial: string
  large: boolean
  sections: ChatSection[]
  exceptId?: string
  onCancel: () => void
  onSubmit: (name: string) => Promise<void>
}) {
  const [value, setValue] = useState(initial)
  const [error, setError] = useState<string | undefined>()
  const [busy, setBusy] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  const submit = async () => {
    const problem = validateSectionName(value, sections, exceptId)
    if (problem) {
      setError(problem)
      input.current?.focus()
      return
    }
    setBusy(true)
    try {
      await onSubmit(value)
    } catch (e) {
      setError(statusOf(e) === 409 ? 'Bạn đã có mục tên này' : explain(e, exceptId ? 'đổi tên mục' : 'tạo mục'))
      input.current?.focus()
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
      className="grid gap-3 p-2"
    >
      <TextField
        ref={input}
        label={label}
        large={large}
        value={value}
        maxLength={MAX_SECTION_NAME}
        placeholder="Ví dụ: Dự án Q4"
        autoComplete="off"
        error={error}
        onChange={(e) => {
          setValue(e.target.value)
          setError(undefined)
        }}
      />
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" size="sm" className={large ? 'min-h-11 px-4' : ''} onClick={onCancel}>
          Huỷ
        </Button>
        <Button type="submit" size="sm" loading={busy} className={large ? 'min-h-11 px-5' : ''}>
          {submitLabel}
        </Button>
      </div>
    </form>
  )
}
