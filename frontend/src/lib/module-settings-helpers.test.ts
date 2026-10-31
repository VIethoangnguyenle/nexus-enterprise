import { afterEach, describe, expect, it } from 'vitest'
import type { DriveItem } from '../api/drive'
import { SORT_OF_SETTING, sortItems } from '../components/drive/drive-model'
import { isMeteredConnection } from '../hooks/useMeteredConnection'
import { calendarShown, monthWeeks, rangeOf, visibleDays } from './calendar-format'
import { orderConversations, type Conversation } from './conversations'

describe('calendar-format: the week start', () => {
  const wed = new Date(2026, 9, 14)
  it('a week starts on Monday by default and on Sunday when asked', () => {
    expect(visibleDays('week', wed)[0]).toEqual(new Date(2026, 9, 12))
    expect(visibleDays('week', wed, 0)[0]).toEqual(new Date(2026, 9, 11))
    expect(rangeOf('week', wed, 0)).toEqual({ from: new Date(2026, 9, 11), to: new Date(2026, 9, 18) })
  })
  it('a month grid starts on the chosen day', () => {
    expect(monthWeeks(wed)[0]![0]).toEqual(new Date(2026, 8, 28))
    expect(monthWeeks(wed, 0)[0]![0]).toEqual(new Date(2026, 8, 27))
  })
})

describe('calendarShown', () => {
  const on = { id: 'a', visible: true }
  it('an empty or absent list is every calendar its checkbox shows', () => {
    expect(calendarShown(on, [])).toBe(true)
    expect(calendarShown(on, null)).toBe(true)
    expect(calendarShown(on, undefined)).toBe(true)
    expect(calendarShown({ id: 'a', visible: false }, [])).toBe(false)
  })
  it('a list shows only what it names, and never what the checkbox hides', () => {
    expect(calendarShown(on, ['a'])).toBe(true)
    expect(calendarShown(on, ['b'])).toBe(false)
    expect(calendarShown({ id: 'a', visible: false }, ['a'])).toBe(false)
  })
})

describe('sortItems: Thư mục trước tệp', () => {
  const base: DriveItem = {
    id: 'i', workspace_id: 'w', drive_context: 'workspace', item_type: 'file', name: 'a',
    ngac_node_id: 'n', owner_id: 'o', status: 'active', created_at: '', updated_at: '',
  }
  const items = [{ ...base, name: 'b.txt' }, { ...base, name: 'a', item_type: 'folder' as const }, { ...base, name: 'c.txt' }]
  it('puts folders first by default, and mixes them in by name when off', () => {
    expect(sortItems(items, 'name').map((i) => i.name)).toEqual(['a', 'b.txt', 'c.txt'])
    const mixed = [{ ...base, name: 'b.txt' }, { ...base, name: 'z', item_type: 'folder' as const }, { ...base, name: 'a.txt' }]
    expect(sortItems(mixed, 'name').map((i) => i.name)).toEqual(['z', 'a.txt', 'b.txt'])
    expect(sortItems(mixed, 'name', false).map((i) => i.name)).toEqual(['a.txt', 'b.txt', 'z'])
  })
  it('reads the settings order as the screen names it', () => {
    expect(SORT_OF_SETTING).toEqual({ modified: 'recent', name: 'name', size: 'size' })
  })
})

describe('orderConversations', () => {
  const c = (title: string): Conversation => ({ id: title, kind: 'space', title, hueKey: title, unread: 0, memberCount: 2, lastActivity: 0 })
  const list = [c('Vận hành'), c('Đối soát'), c('An toàn')]
  it('activity keeps the order it was given; name sorts the Vietnamese way', () => {
    expect(orderConversations(list, 'activity')).toBe(list)
    expect(orderConversations(list, 'name').map((x) => x.title)).toEqual(['An toàn', 'Đối soát', 'Vận hành'])
  })
})

describe('isMeteredConnection', () => {
  const set = (c: object | undefined) => Object.defineProperty(navigator, 'connection', { configurable: true, value: c })
  afterEach(() => set(undefined))
  it('is true on mobile data or with data saving, false otherwise or when the browser says nothing', () => {
    set(undefined)
    expect(isMeteredConnection()).toBe(false)
    set({ type: 'wifi' })
    expect(isMeteredConnection()).toBe(false)
    set({ type: 'cellular' })
    expect(isMeteredConnection()).toBe(true)
    set({ type: 'wifi', saveData: true })
    expect(isMeteredConnection()).toBe(true)
  })
})
