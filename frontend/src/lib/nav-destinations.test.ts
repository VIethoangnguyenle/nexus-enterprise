import { describe, expect, it } from 'vitest'
import {
  barModules, chatTabOf, defaultBar, footDestinations, footNavItems, isModuleBarPath, isPhoneDetailPath, mainDestinations,
  moduleOfPath, phoneNameOf, resolveBar, searchHintOf, validateChannelsSearch, chatOverlayOf,
} from './nav-destinations'

describe('destinations by workspace type', () => {
  it('offers the six main destinations to an organization, Lịch right after Tin nhắn', () => {
    expect(mainDestinations(false).map((d) => d.to)).toEqual(['/channels', '/calendar', '/drive', '/approval', '/assets', '/contacts'])
  })

  it('leaves out Phê duyệt and Tài sản in a personal workspace', () => {
    expect(mainDestinations(true).map((d) => d.id)).toEqual(['messaging', 'calendar', 'drive', 'contacts'])
  })

  it('keeps Quản trị and Cài đặt at the foot', () => {
    expect(footNavItems.map((d) => d.label)).toEqual(['Quản trị', 'Cài đặt'])
    expect(footDestinations(false).map((d) => d.label)).toEqual(['Quản trị', 'Cài đặt'])
  })

  it('leaves Quản trị out of a personal workspace', () => {
    expect(footDestinations(true).map((d) => d.label)).toEqual(['Cài đặt'])
  })

  it('names Lịch with the Calendar icon, in an organization and in a personal workspace', () => {
    for (const personal of [false, true]) {
      const lich = mainDestinations(personal).find((d) => d.id === 'calendar')
      expect(lich).toMatchObject({ label: 'Lịch', to: '/calendar' })
      expect(lich?.icon).toBeDefined()
    }
  })

  it('knows the calendar paths as the Lịch module, with a search hint of its own', () => {
    expect(moduleOfPath('/calendar')).toBe('calendar')
    expect(moduleOfPath('/calendar/')).toBe('calendar')
    expect(searchHintOf('calendar')).toBe('Tìm sự kiện')
    expect(phoneNameOf('calendar')).toBe('Lịch')
  })

  it('calls the first destination Chat on the phone and Tin nhắn on the desktop', () => {
    const chat = mainDestinations(false)[0]!
    expect(chat.label).toBe('Tin nhắn')
    expect(chat.phoneLabel).toBe('Chat')
  })
})

describe('moduleOfPath', () => {
  it.each([
    ['/channels', 'messaging'],
    ['/channels/abc', 'messaging'],
    ['/drive', 'drive'],
    ['/documents/abc', 'drive'],
    ['/approval', 'approval'],
    ['/assets', 'assets'],
    ['/contacts', 'contacts'],
    ['/admin/users', 'admin'],
    ['/settings', 'settings'],
  ])('%s belongs to %s', (path, module) => expect(moduleOfPath(path)).toBe(module))

  it('knows nothing of a path outside the modules', () => {
    expect(moduleOfPath('/workspace-select')).toBeUndefined()
  })
})

describe('the search bar hint', () => {
  it.each([
    ['messaging', 'Tìm trong chat'], ['drive', 'Tìm tài liệu'], ['approval', 'Tìm đề nghị'],
    ['assets', 'Tìm tài sản'], ['contacts', 'Tìm người'], ['settings', 'Tìm cài đặt'], ['admin', 'Tìm cài đặt'],
  ] as const)('%s says "%s"', (module, hint) => expect(searchHintOf(module)).toBe(hint))

  it('names the module for a screen without a search of its own', () => {
    expect(phoneNameOf('messaging')).toBe('Chat')
    expect(phoneNameOf('approval')).toBe('Phê duyệt')
  })
})

describe('where the phone shell steps aside', () => {
  it.each(['/channels', '/channels/', '/drive', '/approval', '/assets', '/contacts', '/calendar', '/documents'])(
    'shows the bottom bar on the list screen %s',
    (path) => expect(isModuleBarPath(path)).toBe(true),
  )

  it.each([
    '/channels/c-1', '/channels/mentions', '/channels/starred', '/documents/d-1', '/calendar/new', '/settings',
    '/settings/calendar', '/admin', '/admin/users',
  ])(
    'hides the bottom bar on the detail, settings or administration screen %s',
    (path) => expect(isModuleBarPath(path)).toBe(false),
  )

  it('hides the search bar inside a conversation or a document, not on their lists', () => {
    expect(isPhoneDetailPath('/channels/c-1')).toBe(true)
    expect(isPhoneDetailPath('/documents/d-1')).toBe(true)
    expect(isPhoneDetailPath('/documents')).toBe(false)
    expect(isPhoneDetailPath('/channels')).toBe(false)
    expect(isPhoneDetailPath('/approval')).toBe(false)
  })
})

describe('the Chat chip in the address', () => {
  it('uses the remembered chip, Tất cả by default, when the address names none', () => {
    expect(chatTabOf(validateChannelsSearch({}))).toBe('all')
    expect(chatTabOf(validateChannelsSearch({}), 'sections')).toBe('sections')
  })

  it.each(['all', 'sections', 'unread', 'mentions', 'direct', 'spaces'] as const)('keeps %s and lets it win over the remembered chip', (tab) => {
    expect(chatTabOf(validateChannelsSearch({ tab }), 'direct')).toBe(tab)
  })

  it('reads an old link to Thêm as Được nhắc', () => {
    expect(validateChannelsSearch({ tab: 'more' }).tab).toBe('mentions')
    expect(chatTabOf({ tab: 'more' })).toBe('mentions')
  })

  it('falls back to the remembered chip for a value it does not know, without keeping it', () => {
    const search = validateChannelsSearch({ tab: 'nope' })
    expect(search.tab).toBeUndefined()
    expect(chatTabOf(search, 'unread')).toBe('unread')
    expect(chatTabOf({ tab: 42 })).toBe('all')
  })

  it('does not write Trang chủ, the name of the old default, into the address, and keeps the workspace', () => {
    expect(validateChannelsSearch({ tab: 'home', ws: 'w1' })).toEqual({ ws: 'w1' })
  })
})

describe('the bottom bar', () => {
  it('offers every main destination and Thông báo in an organization', () => {
    expect(barModules(false).map((m) => m.id)).toEqual(['chat', 'calendar', 'drive', 'approval', 'assets', 'contacts', 'notifications'])
  })

  it('never offers Phê duyệt or Tài sản in a personal workspace', () => {
    expect(barModules(true).map((m) => m.id)).toEqual(['chat', 'calendar', 'drive', 'contacts', 'notifications'])
  })

  it('never offers Quản trị anywhere', () => {
    for (const personal of [false, true]) expect(barModules(personal).map((m) => m.id)).not.toContain('admin')
  })

  it('defaults to Chat · Lịch · Tài liệu · Phê duyệt, with Danh bạ for Phê duyệt in a personal workspace', () => {
    expect(defaultBar(false)).toEqual(['chat', 'calendar', 'drive', 'approval'])
    expect(defaultBar(true)).toEqual(['chat', 'calendar', 'drive', 'contacts'])
  })

  it('keeps the stored choice, in its order, when it is four different modules the workspace offers', () => {
    expect(resolveBar(['assets', 'chat', 'notifications', 'drive'], false)).toEqual(['assets', 'chat', 'notifications', 'drive'])
  })

  it.each([
    ['unset', undefined],
    ['not a list', 'chat'],
    ['three slots', ['chat', 'calendar', 'drive']],
    ['five slots', ['chat', 'calendar', 'drive', 'approval', 'assets']],
    ['a repeat', ['chat', 'chat', 'drive', 'approval']],
    ['an unknown module', ['chat', 'calendar', 'drive', 'admin']],
  ])('falls back to the defaults for %s', (_name, stored) => {
    expect(resolveBar(stored, false)).toEqual(defaultBar(false))
  })

  it('falls back to the defaults when a stored slot is not offered in a personal workspace', () => {
    expect(resolveBar(['chat', 'calendar', 'drive', 'approval'], true)).toEqual(defaultBar(true))
    expect(resolveBar(['chat', 'calendar', 'assets', 'contacts'], true)).toEqual(defaultBar(true))
  })
})

describe('the full-screen layers of the Chat list in the address', () => {
  it('keeps the search screen for 1, "1" and true, and drops anything else', () => {
    for (const v of [1, '1', true]) expect(validateChannelsSearch({ search: v }).search).toBe(1)
    for (const v of [0, '0', 'yes', false, 2, null, {}]) expect(validateChannelsSearch({ search: v }).search).toBeUndefined()
  })

  it('keeps the words and filters of the search screen only while the screen is open, and only well-formed ones', () => {
    const address = { q: ' đối soát ', from: 'u-1', in: 'c-1', since: '2026-10-01', until: '2026-13-40', file: '1', link: 0, me: 1 }
    expect(validateChannelsSearch({ search: 1, ...address })).toEqual({
      search: 1, q: 'đối soát', from: 'u-1', in: 'c-1', since: '2026-10-01', file: 1, me: 1,
    })
    expect(validateChannelsSearch(address)).toEqual({})
  })

  it('keeps a Thread list that exists and drops one that does not', () => {
    expect(validateChannelsSearch({ threads: 'new' }).threads).toBe('new')
    expect(validateChannelsSearch({ threads: 'followed' }).threads).toBe('followed')
    expect(validateChannelsSearch({ threads: 'all' }).threads).toBeUndefined()
    expect(validateChannelsSearch({ threads: 7 }).threads).toBeUndefined()
  })

  it('names the layer to show; the search screen wins when both are asked for', () => {
    expect(chatOverlayOf({})).toBeNull()
    expect(chatOverlayOf({ search: 1 })).toEqual({ kind: 'search' })
    expect(chatOverlayOf({ threads: 'new' })).toEqual({ kind: 'threads', chip: 'unread' })
    expect(chatOverlayOf({ threads: 'followed' })).toEqual({ kind: 'threads', chip: 'following' })
    expect(chatOverlayOf({ search: 1, threads: 'new' })).toEqual({ kind: 'search' })
    expect(chatOverlayOf({ search: 'x', threads: 'x' })).toBeNull()
  })

  it('keeps the tab and the workspace beside a layer', () => {
    expect(validateChannelsSearch({ tab: 'direct', ws: 'w1', threads: 'followed' }))
      .toEqual({ tab: 'direct', ws: 'w1', threads: 'followed' })
  })
})
