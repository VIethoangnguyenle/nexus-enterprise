import { useEffect, useState } from 'react'
import { useCalendarSettings, useUpdateSettings } from '../../hooks/useCalendarEditor'
import { minutesOfClock } from '../../lib/calendar-findtime'
import { Button, FieldInput, FieldSelect } from '../primitives'
import { Dialog } from '../composites/Dialog'
import { zoneLabel, zoneNames } from './editor-ui'

/** Minutes before a new event that the default reminder can be. 0 is no reminder. */
const REMINDERS: { minutes: number; label: string }[] = [
  { minutes: 0, label: 'Không nhắc' },
  { minutes: 5, label: '5 phút trước' },
  { minutes: 10, label: '10 phút trước' },
  { minutes: 15, label: '15 phút trước' },
  { minutes: 30, label: '30 phút trước' },
  { minutes: 60, label: '1 giờ trước' },
  { minutes: 1440, label: '1 ngày trước' },
]

interface Props {
  open: boolean
  wsId: string
  onClose: () => void
}

/** Cài đặt lịch (DESIGN.md §6): the time zone, working hours (Tìm giờ greys the rest) and the default reminder. */
export function CalendarSettingsDialog({ open, wsId, onClose }: Props) {
  const settings = useCalendarSettings(wsId)
  const save = useUpdateSettings(wsId)
  const [tzid, setTzid] = useState('')
  const [workStart, setWorkStart] = useState('08:00')
  const [workEnd, setWorkEnd] = useState('17:00')
  const [reminder, setReminder] = useState(10)

  useEffect(() => {
    if (!open || !settings.data) return
    setTzid(settings.data.tzid)
    setWorkStart(settings.data.workStart)
    setWorkEnd(settings.data.workEnd)
    setReminder(settings.data.defaultReminder)
  }, [open, settings.data])

  const hoursWrong = minutesOfClock(workEnd, 0) <= minutesOfClock(workStart, 0)
  const ready = !!settings.data && !!tzid
  const reminders = REMINDERS.some((r) => r.minutes === reminder) ? REMINDERS : [...REMINDERS, { minutes: reminder, label: `${reminder} phút trước` }]

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Cài đặt lịch"
      width="sm"
      onSubmit={() => {
        if (!ready || hoursWrong) return
        save.mutate({ tzid, workStart, workEnd, defaultReminder: reminder }, { onSuccess: onClose })
      }}
      footer={
        <>
          <Button type="button" variant="soft" className="rounded-full" onClick={onClose}>Huỷ</Button>
          <Button type="submit" className="rounded-full" loading={save.isPending} disabled={!ready || hoursWrong}>Lưu</Button>
        </>
      }
    >
      {settings.isError ? (
        <p role="alert" className="m-0 text-sm text-ink-muted">Chưa tải được cài đặt lịch. Thử lại sau ít phút.</p>
      ) : (
        <div className="grid gap-4">
          <label className="grid gap-1.5 text-sm font-semibold text-ink">
            Múi giờ
            <FieldSelect value={tzid} onChange={(e) => setTzid(e.target.value)} disabled={!settings.data} className="w-full">
              {(tzid ? zoneNames(tzid) : []).map((z) => <option key={z} value={z}>{zoneLabel(z)}</option>)}
            </FieldSelect>
          </label>
          <fieldset className="grid gap-1.5 border-none p-0 m-0">
            <legend className="text-sm font-semibold text-ink pb-1.5">Giờ làm việc</legend>
            <div className="flex items-center gap-2 text-sm text-ink">
              <FieldInput type="time" aria-label="Bắt đầu làm việc" value={workStart} onChange={(e) => setWorkStart(e.target.value)} invalid={hoursWrong} />
              <span className="text-ink-muted">đến</span>
              <FieldInput type="time" aria-label="Kết thúc làm việc" value={workEnd} onChange={(e) => setWorkEnd(e.target.value)} invalid={hoursWrong} />
            </div>
            {hoursWrong && <span className="text-xs font-medium text-danger" role="alert">Giờ kết thúc phải sau giờ bắt đầu</span>}
            <span className="text-xs text-ink-muted">Tìm giờ chỉ gợi ý trong giờ làm việc của mọi người.</span>
          </fieldset>
          <label className="grid gap-1.5 text-sm font-semibold text-ink">
            Lời nhắc mặc định
            <FieldSelect value={reminder} onChange={(e) => setReminder(Number(e.target.value))} disabled={!settings.data} className="w-full">
              {reminders.map((r) => <option key={r.minutes} value={r.minutes}>{r.label}</option>)}
            </FieldSelect>
          </label>
        </div>
      )}
    </Dialog>
  )
}
