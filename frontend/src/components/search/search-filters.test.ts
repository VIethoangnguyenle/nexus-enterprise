import { describe, expect, it } from 'vitest'
import type { Occurrence } from '../../api/calendar'
import type { DriveItem } from '../../api/drive'
import { approvalQuery, driveKindOf, filterDrive, filterEvents } from './search-filters'

const NOW = new Date(2026, 9, 19, 10, 0)
const item = (name: string, over: Partial<DriveItem> = {}): DriveItem => ({
  id: name, workspace_id: 'w', drive_context: 'workspace', item_type: 'file', name, ngac_node_id: 'n', owner_id: 'u1',
  status: 'active', created_at: NOW.toISOString(), updated_at: NOW.toISOString(), ...over,
})
const event = (start: Date, hours: number, calendarId = 'c1'): Occurrence => ({
  eventId: start.toISOString(), occurrenceStart: start.toISOString(), start: start.toISOString(),
  end: new Date(start.getTime() + hours * 3_600_000).toISOString(), allDay: false, calendarId, title: 'Họp',
  busyOnly: false, isOrganizer: false, hasCall: false, isRecurring: false, canEdit: false, kind: 'event', guestCount: 0, version: 1,
})

describe('Tài liệu chips', () => {
  const items = [
    item('a.xlsx'), item('b.docx', { owner_id: 'u2' }), item('Hồ sơ', { item_type: 'folder', parent_id: 'f1' }),
    item('cũ.pdf', { updated_at: new Date(2026, 8, 1).toISOString() }),
  ]

  it('names folders and file kinds', () => {
    expect(items.map(driveKindOf)).toEqual(['Bảng tính', 'Văn bản', 'Thư mục', 'PDF'])
  })

  it('narrows by kind, owner, folder and date, all together', () => {
    expect(filterDrive(items, { kind: 'Bảng tính' }, NOW).map((i) => i.name)).toEqual(['a.xlsx'])
    expect(filterDrive(items, { editor: 'u2' }, NOW).map((i) => i.name)).toEqual(['b.docx'])
    expect(filterDrive(items, { folder: 'f1' }, NOW).map((i) => i.name)).toEqual(['Hồ sơ'])
    expect(filterDrive(items, { folder: '' }, NOW).map((i) => i.name)).toEqual(['a.xlsx', 'b.docx', 'cũ.pdf'])
    expect(filterDrive(items, { date: '7d' }, NOW).map((i) => i.name)).toEqual(['a.xlsx', 'b.docx', 'Hồ sơ'])
    expect(filterDrive(items, { date: '7d', kind: 'PDF' }, NOW)).toEqual([])
    expect(filterDrive(items, {}, NOW)).toHaveLength(4)
  })
})

describe('Lịch chips', () => {
  const today = event(new Date(2026, 9, 19, 15), 1)
  const nextWeek = event(new Date(2026, 9, 24, 9), 1, 'c2')
  const nextMonth = event(new Date(2026, 10, 10, 9), 1)

  it('narrows by calendar and by the days ahead', () => {
    expect(filterEvents([today, nextWeek, nextMonth], { calendar: 'c2' }, NOW)).toEqual([nextWeek])
    expect(filterEvents([today, nextWeek, nextMonth], { range: 'today' }, NOW)).toEqual([today])
    expect(filterEvents([today, nextWeek, nextMonth], { range: 'next7' }, NOW)).toEqual([today, nextWeek])
    expect(filterEvents([today, nextWeek, nextMonth], { range: 'next30' }, NOW)).toEqual([today, nextWeek, nextMonth])
  })
})

describe('Phê duyệt chips', () => {
  it('go to the server as its parameters, days in the reader\'s zone', () => {
    expect(approvalQuery({ status: 'pending', template: 't1', sender: 'u1', date: '7d' }, NOW)).toEqual({
      status: 'pending', template_id: 't1', created_by: 'u1', from: '2026-10-13', to: '2026-10-19',
    })
    expect(approvalQuery({}, NOW)).toEqual({ status: undefined, template_id: undefined, created_by: undefined, from: undefined, to: undefined })
  })
})
