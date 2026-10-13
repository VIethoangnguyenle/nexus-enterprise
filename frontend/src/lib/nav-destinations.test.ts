import { describe, expect, it } from 'vitest'
import {
  chatTabOf, footDestinations, footNavItems, isChatListPath, isPhoneDetailPath, mainDestinations, moduleOfPath, phoneNameOf,
  searchHintOf, validateChannelsSearch,
} from './nav-destinations'

describe('destinations by workspace type', () => {
  it('offers the five main destinations to an organization, in order', () => {
    expect(mainDestinations(false).map((d) => d.to)).toEqual(['/channels', '/drive', '/approval', '/assets', '/contacts'])
  })

  it('leaves out Phê duyệt and Tài sản in a personal workspace', () => {
    expect(mainDestinations(true).map((d) => d.id)).toEqual(['messaging', 'drive', 'contacts'])
  })

  it('keeps Quản trị and Cài đặt at the foot', () => {
    expect(footNavItems.map((d) => d.label)).toEqual(['Quản trị', 'Cài đặt'])
    expect(footDestinations(false).map((d) => d.label)).toEqual(['Quản trị', 'Cài đặt'])
  })

  it('leaves Quản trị out of a personal workspace', () => {
    expect(footDestinations(true).map((d) => d.label)).toEqual(['Cài đặt'])
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
  it('shows the Chat bottom bar on the conversation list only', () => {
    expect(isChatListPath('/channels')).toBe(true)
    expect(isChatListPath('/channels/')).toBe(true)
    expect(isChatListPath('/channels/c-1')).toBe(false)
    expect(isChatListPath('/channels/mentions')).toBe(false)
    expect(isChatListPath('/drive')).toBe(false)
  })

  it('hides the search bar inside a conversation or a document, not on their lists', () => {
    expect(isPhoneDetailPath('/channels/c-1')).toBe(true)
    expect(isPhoneDetailPath('/documents/d-1')).toBe(true)
    expect(isPhoneDetailPath('/documents')).toBe(false)
    expect(isPhoneDetailPath('/channels')).toBe(false)
    expect(isPhoneDetailPath('/approval')).toBe(false)
  })
})

describe('the Chat tab in the address', () => {
  it('opens Trang chủ when the address names no tab', () => {
    expect(chatTabOf(validateChannelsSearch({}))).toBe('home')
  })

  it.each(['direct', 'sections', 'more'] as const)('keeps %s', (tab) => {
    expect(chatTabOf(validateChannelsSearch({ tab }))).toBe(tab)
  })

  it('falls back to Trang chủ for a value it does not know, without keeping it', () => {
    const search = validateChannelsSearch({ tab: 'nope' })
    expect(search.tab).toBeUndefined()
    expect(chatTabOf(search)).toBe('home')
    expect(chatTabOf({ tab: 42 })).toBe('home')
  })

  it('does not write the default into the address, and keeps the workspace', () => {
    expect(validateChannelsSearch({ tab: 'home', ws: 'w1' })).toEqual({ ws: 'w1' })
  })
})
