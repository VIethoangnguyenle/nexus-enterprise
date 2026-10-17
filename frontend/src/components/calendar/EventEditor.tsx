import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { addDays } from 'date-fns'
import { ArrowLeft, ChevronRight, Users, X } from 'lucide-react'
import type { CalendarSummary, EditScope, EventDetail } from '../../api/calendar'
import { useCalendarActions } from '../../hooks/useCalendar'
import { useCalendarSettings, useFreeBusy, useSaveEvent } from '../../hooks/useCalendarEditor'
import { usePeople } from '../../hooks/usePeople'
import { usePhone } from '../../hooks/usePhone'
import {
  hasChanges, isDirty, remindersShown, spanOfForm, toInput, toPatch, validateForm, withSpan, formDuration,
  type EditorForm, type GuestPick,
} from '../../lib/calendar-editor'
import { spansOf, type Span } from '../../lib/calendar-findtime'
import { parseDateKey, toDateKey } from '../../lib/calendar-format'
import { useAuthStore } from '../../stores/auth.store'
import { Button, FieldInput, IconButton, Pressable, TabBar } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { EditorFields } from './EditorFields'
import { EditorWhen } from './EditorWhen'
import { FindTime } from './FindTime'
import { GuestsPanel, guestFromPerson } from './GuestsPanel'
import { RecurrenceScopeDialog } from './RecurrenceScopeDialog'

const TABS = [
  { id: 'details', label: 'Chi tiết sự kiện' },
  { id: 'find', label: 'Tìm giờ' },
]

interface Props {
  wsId: string
  /** The form as it starts; the editor keeps its own copy to change. */
  initial: EditorForm
  /** The event being edited, absent for a new one. */
  original?: EventDetail
  /** The instance being edited, for the scope of a recurring change. */
  occurrence?: string
  calendars: CalendarSummary[]
  /** The creator of a new event, or the organizer of an existing one. */
  organizer?: GuestPick
  /** Loads the event again and starts the editor over on it (after somebody else saved first). */
  onReload?: () => void
}

/**
 * The full editor (design/mockups/calendar.html §3, §4): a route of its own over the content area, so Back
 * returns to the grid. Title and Lưu on top, the time and repeat under them, then two tabs: Chi tiết sự kiện
 * (fields, guests with whether they are free) and Tìm giờ. Ctrl+S saves; closing with changes asks "Bỏ thay
 * đổi?"; a recurring event asks which occurrences first. On a phone the guests are a screen of their own.
 */
export function EventEditor({ wsId, initial, original, occurrence, calendars, organizer, onReload }: Props) {
  const navigate = useNavigate()
  const phone = usePhone()
  const actions = useCalendarActions(wsId)
  const saveEvent = useSaveEvent(wsId)
  const settings = useCalendarSettings(wsId)
  const people = usePeople(wsId)
  const meUserId = useAuthStore((s) => s.user?.id)

  const [form, setForm] = useState(initial)
  const base = useRef(initial)
  const [tab, setTab] = useState<'details' | 'find'>('details')
  const [guestsOpen, setGuestsOpen] = useState(false)
  const [left, setLeft] = useState({ start: false, end: false })
  const [attempted, setAttempted] = useState(false)
  const [saving, setSaving] = useState(false)
  const [scopeOpen, setScopeOpen] = useState(false)
  const [discardOpen, setDiscardOpen] = useState(false)
  const [conflict, setConflict] = useState(false)
  const titleRef = useRef<HTMLInputElement>(null)

  const patch = useCallback((p: Partial<EditorForm>) => setForm((f) => ({ ...f, ...p })), [])
  const errors = useMemo(() => validateForm(form), [form])
  const show = { start: attempted || left.start, end: attempted || left.end }
  const span = useMemo(() => spanOfForm(form), [form])
  const dirty = isDirty(form, base.current)
  const defaultReminder = settings.data?.defaultReminder ?? 0

  const me = meUserId ? people.byUserId.get(meUserId) : undefined
  const self: GuestPick | undefined = organizer ?? (me ? guestFromPerson(me) : undefined)

  const leave = useCallback(
    (dateKey?: string) =>
      void navigate({
        to: '/calendar',
        search: (prev: { ws?: string }) => ({ ws: prev.ws, ...(dateKey ? { date: dateKey } : {}) }),
      }),
    [navigate],
  )
  const close = useCallback(() => (dirty ? setDiscardOpen(true) : leave(form.startDate)), [dirty, leave, form.startDate])

  // --- Free/busy of the guests at the chosen time ---------------------------------------------
  const dayStart = useMemo(() => parseDateKey(form.startDate), [form.startDate])
  const guestNodes = useMemo(() => form.guests.map((g) => g.nodeId), [form.guests])
  const busyQuery = useFreeBusy(wsId, guestNodes, span?.start ?? null, span?.end ?? null)
  const busy = useMemo(() => {
    const out = new Map<string, Span[]>()
    // Everyone asked about is known to have no clash unless the server lists a span for them.
    if (busyQuery.data) for (const n of guestNodes) out.set(n, [])
    for (const p of busyQuery.data ?? []) out.set(p.person.nodeId, spansOf(p.busy))
    return out
  }, [busyQuery.data, guestNodes])

  // --- Saving ---------------------------------------------------------------------------------
  const run = useCallback(
    async (scope?: EditScope) => {
      const input = toInput(form)
      if (!input || Object.keys(validateForm(form)).length > 0) {
        setAttempted(true)
        return
      }
      setSaving(true)
      try {
        if (!original) {
          const saved = await actions.create(input)
          if (saved) leave(form.startDate)
          return
        }
        const changes = toPatch(form, base.current, original.version)
        if (!changes || !hasChanges(changes)) {
          leave(form.startDate)
          return
        }
        const result = await saveEvent(original.eventId, changes, scope ? { scope, occurrence: occurrence ?? original.occurrenceStart } : {})
        if (result.ok) leave(form.startDate)
        else if (result.reason === 'conflict') setConflict(true)
      } finally {
        setSaving(false)
      }
    },
    [form, original, actions, saveEvent, leave, occurrence],
  )

  const submit = useCallback(() => {
    if (saving) return
    if (Object.keys(validateForm(form)).length > 0) {
      setAttempted(true)
      return
    }
    // Only a change that is not a no-op needs the question.
    if (original?.isRecurring && hasChanges(toPatch(form, base.current, original.version) ?? { version: 0 })) setScopeOpen(true)
    else void run()
  }, [saving, form, original, run])

  // Ctrl+S (or ⌘S) saves from anywhere in the editor.
  const latestSubmit = useRef(submit)
  useEffect(() => {
    latestSubmit.current = submit
  })
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === 's') {
        e.preventDefault()
        latestSubmit.current()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => {
    if (!original) titleRef.current?.focus()
  }, [original])

  const columns = useMemo(
    () => [
      ...(self ? [{ guest: self, isMe: true }] : []),
      ...form.guests.map((g) => ({ guest: g, isMe: false })),
    ],
    [self, form.guests],
  )
  const hasOthers = form.guests.length > 0 || !!form.inviteChannelId

  const guestsPanel = (
    <GuestsPanel wsId={wsId} form={form} onChange={patch} organizer={original ? organizer : self} busy={busy} span={span} />
  )
  const fields = (
    <EditorFields
      form={form}
      onChange={patch}
      calendars={calendars}
      reminders={remindersShown(form, defaultReminder)}
      onReminders={(next) => patch({ reminders: next })}
    />
  )

  return (
    <div className="flex-1 min-h-0 min-w-0 flex flex-col bg-base" data-testid="event-editor">
      <header className="flex items-center gap-2 px-3 pt-3 pb-2 lg:px-5">
        <IconButton size="lg" shape="round" aria-label={phone ? 'Quay lại' : 'Đóng trình soạn'} onClick={close}>
          {phone ? <ArrowLeft size={20} strokeWidth={1.75} /> : <X size={20} strokeWidth={1.75} />}
        </IconButton>
        <FieldInput
          ref={titleRef}
          aria-label="Tiêu đề"
          placeholder="Thêm tiêu đề"
          value={form.title}
          maxLength={200}
          autoComplete="off"
          onChange={(e) => patch({ title: e.target.value })}
          className="flex-1 h-12 text-xl"
        />
        <Button type="button" className="rounded-full px-6" loading={saving} onClick={submit}>Lưu</Button>
      </header>

      {conflict && (
        <div role="alert" className="flex flex-wrap items-center gap-3 mx-4 lg:mx-5 mb-2 px-4 py-2.5 rounded-lg bg-warning-wash text-sm text-ink">
          <span className="flex-1">Có người vừa sửa sự kiện này. Hãy xem bản mới trước khi lưu.</span>
          {onReload && <Button type="button" variant="soft" size="sm" className="rounded-full" onClick={onReload}>Xem bản mới</Button>}
        </div>
      )}

      <div className="px-4 lg:px-5 pb-3 lg:pl-17">
        <EditorWhen
          form={form}
          onChange={setForm}
          errors={errors}
          show={show}
          onBlurField={(f) => setLeft((l) => ({ ...l, [f]: true }))}
        />
      </div>

      <TabBar tabs={TABS} value={tab} onChange={(id) => setTab(id as 'details' | 'find')} label="Trình soạn" idPrefix="editor" className="px-3 lg:px-5 border-b border-line" />

      <div className="flex-1 min-h-0 overflow-y-auto flex flex-col">
        {tab === 'details' ? (
          <div
            role="tabpanel" id="editor-panel-details" aria-labelledby="editor-tab-details"
            className="grid gap-6 px-4 py-4 lg:px-5 lg:grid-cols-[minmax(0,1fr)_21rem]"
          >
            {fields}
            {phone ? (
              <Pressable
                onClick={() => setGuestsOpen(true)}
                className="flex items-center gap-3 min-h-14 px-3 rounded-lg bg-sunk text-left text-16 text-ink"
              >
                <Users size={20} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />
                <span className="flex-1">{hasOthers ? `Khách · ${form.guests.length + (form.inviteChannelId ? 1 : 0)}` : 'Thêm khách'}</span>
                <ChevronRight size={18} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />
              </Pressable>
            ) : guestsPanel}
          </div>
        ) : (
          <div role="tabpanel" id="editor-panel-find" aria-labelledby="editor-tab-find" className="flex-1 min-h-0 flex flex-col pt-3">
            {dayStart && (
              <FindTime
                wsId={wsId}
                day={dayStart}
                span={span}
                minutes={Math.max(15, formDuration(form) || 60)}
                columns={columns}
                onSpan={(s, e) => setForm((f) => withSpan(f, s, e))}
                onShiftDay={(n) => setForm((f) => {
                  const a = parseDateKey(f.startDate)
                  const b = parseDateKey(f.endDate)
                  return a && b ? { ...f, startDate: toDateKey(addDays(a, n)), endDate: toDateKey(addDays(b, n)) } : f
                })}
              />
            )}
          </div>
        )}
      </div>

      {phone && guestsOpen && (
        <div role="dialog" aria-modal="true" aria-label="Khách" className="fixed inset-0 z-modal bg-base flex flex-col">
          <header className="flex items-center gap-2 px-3 pt-3 pb-2">
            <IconButton size="lg" shape="round" aria-label="Quay lại" onClick={() => setGuestsOpen(false)}><ArrowLeft size={20} strokeWidth={1.75} /></IconButton>
            <span className="flex-1 text-lg font-semibold text-ink">Khách</span>
            <Button type="button" variant="soft" className="rounded-full" onClick={() => { setTab('find'); setGuestsOpen(false) }}>Tìm giờ</Button>
          </header>
          <div className="flex-1 overflow-y-auto px-4 pb-6">{guestsPanel}</div>
        </div>
      )}

      <RecurrenceScopeDialog
        open={scopeOpen}
        title="Sửa sự kiện lặp lại"
        onClose={() => setScopeOpen(false)}
        onConfirm={(scope) => {
          setScopeOpen(false)
          void run(scope)
        }}
      />
      <ConfirmDialog
        open={discardOpen}
        onClose={() => setDiscardOpen(false)}
        onConfirm={() => leave(initial.startDate)}
        title="Bỏ thay đổi?"
        description="Những gì bạn đã sửa trong sự kiện này sẽ không được lưu."
        confirmLabel="Bỏ"
        cancelLabel="Tiếp tục sửa"
        confirmVariant="danger"
      />
    </div>
  )
}
