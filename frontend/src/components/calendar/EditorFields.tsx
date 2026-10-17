import { AlignLeft, Bell, Calendar, Eye, MapPin, Plus, Video, X } from 'lucide-react'
import type { CalendarSummary, Transparency, Visibility } from '../../api/calendar'
import {
  REMINDER_UNIT_LABEL, joinMinutes, splitMinutes, type EditorForm, type ReminderUnit,
} from '../../lib/calendar-editor'
import { Button, IconButton, FieldInput, FieldSelect, FieldTextarea } from '../primitives'
import { EditorLine } from './editor-ui'

interface Props {
  form: EditorForm
  onChange: (patch: Partial<EditorForm>) => void
  calendars: CalendarSummary[]
  /** Reminders as shown now (the person's default until they touch them). */
  reminders: number[]
  onReminders: (next: number[]) => void
}

const UNITS = Object.keys(REMINDER_UNIT_LABEL) as ReminderUnit[]

function ReminderRow({ minutes, onChange, onRemove }: { minutes: number; onChange: (m: number) => void; onRemove: () => void }) {
  const { n, unit } = splitMinutes(minutes)
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm text-ink">
      <span>Thông báo</span>
      <FieldInput
        type="number" min={0} aria-label="Số" className="w-16" value={n}
        onChange={(e) => onChange(joinMinutes(Number(e.target.value) || 0, unit))}
      />
      <FieldSelect aria-label="Đơn vị" value={unit} onChange={(e) => onChange(joinMinutes(n, e.target.value as ReminderUnit))}>
        {UNITS.map((u) => <option key={u} value={u}>{REMINDER_UNIT_LABEL[u]}</option>)}
      </FieldSelect>
      <span className="text-ink-muted">trước</span>
      <IconButton aria-label="Bỏ lời nhắc" onClick={onRemove}><X size={16} strokeWidth={1.75} /></IconButton>
    </div>
  )
}

/**
 * The left column of Chi tiết sự kiện (design/mockups/calendar.html §3): the video call, the place, reminders,
 * the calendar, how the time shows and who may see it, and the description.
 */
export function EditorFields({ form, onChange, calendars, reminders, onReminders }: Props) {
  return (
    <div className="grid gap-3 content-start">
      <EditorLine icon={<Video size={20} strokeWidth={1.75} />}>
        {form.hasCall ? (
          <div className="flex items-center gap-3 rounded-lg bg-sunk px-3 py-2">
            <span className="grid min-w-0 flex-1 leading-tight">
              <b className="text-sm font-semibold text-ink">Cuộc gọi video Nexus</b>
              <small className="text-xs text-ink-muted">Phòng gọi riêng của sự kiện · chỉ khách vào được · mở từ 15 phút trước</small>
            </span>
            <IconButton aria-label="Bỏ cuộc gọi video" onClick={() => onChange({ hasCall: false })}><X size={16} strokeWidth={1.75} /></IconButton>
          </div>
        ) : (
          <Button type="button" variant="soft" size="sm" className="rounded-full" onClick={() => onChange({ hasCall: true })}>
            <Plus size={16} strokeWidth={1.75} aria-hidden="true" />Thêm cuộc gọi video Nexus
          </Button>
        )}
      </EditorLine>

      <EditorLine icon={<MapPin size={20} strokeWidth={1.75} />}>
        <FieldInput
          aria-label="Địa điểm" placeholder="Thêm địa điểm" className="w-full" value={form.location} maxLength={200}
          onChange={(e) => onChange({ location: e.target.value })}
        />
      </EditorLine>

      <EditorLine icon={<Bell size={20} strokeWidth={1.75} />} top>
        <div className="grid gap-2 pt-0.5">
          {reminders.map((m, i) => (
            <ReminderRow
              key={i}
              minutes={m}
              onChange={(next) => onReminders(reminders.map((x, j) => (j === i ? next : x)))}
              onRemove={() => onReminders(reminders.filter((_, j) => j !== i))}
            />
          ))}
          <Button type="button" variant="ghost" size="sm" className="justify-self-start rounded-full" onClick={() => onReminders([...reminders, 10])}>
            <Plus size={16} strokeWidth={1.75} aria-hidden="true" />Thêm lời nhắc
          </Button>
        </div>
      </EditorLine>

      <EditorLine icon={<Calendar size={20} strokeWidth={1.75} />}>
        <FieldSelect aria-label="Lịch" value={form.calendarId} onChange={(e) => onChange({ calendarId: e.target.value })}>
          {calendars.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </FieldSelect>
      </EditorLine>

      <EditorLine icon={<Eye size={20} strokeWidth={1.75} />}>
        <div className="flex flex-wrap items-center gap-2">
          <FieldSelect aria-label="Hiển thị" value={form.transparency} onChange={(e) => onChange({ transparency: e.target.value as Transparency })}>
            <option value="busy">Hiển thị: Bận</option>
            <option value="free">Hiển thị: Rảnh</option>
          </FieldSelect>
          <FieldSelect aria-label="Quyền xem" value={form.visibility} onChange={(e) => onChange({ visibility: e.target.value as Visibility })}>
            <option value="default">Quyền xem: Mặc định</option>
            <option value="private">Quyền xem: Riêng tư</option>
          </FieldSelect>
        </div>
      </EditorLine>

      <EditorLine icon={<AlignLeft size={20} strokeWidth={1.75} />} top>
        <FieldTextarea
          aria-label="Mô tả"
          placeholder="Thêm mô tả"
          value={form.description}
          onChange={(e) => onChange({ description: e.target.value })}
          rows={5}
          className="min-h-28"
        />
      </EditorLine>
    </div>
  )
}
