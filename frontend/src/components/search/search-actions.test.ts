import { describe, expect, it } from 'vitest'
import { quickActions, typedActions, type ActionContext } from './search-actions'
import { groupsOf, scopeOfModule, scopesFor } from './search-scopes'

const org = (over: Partial<ActionContext> = {}): ActionContext => ({ isPersonal: false, canCreateRequest: true, home: 'all', ...over })
const personal = (over: Partial<ActionContext> = {}): ActionContext => ({ isPersonal: true, canCreateRequest: false, home: 'all', ...over })
const labels = (list: { label: string }[]) => list.map((a) => a.label)

describe('scopes', () => {
  it('start with the module the search was opened in, then Tất cả, then the other modules', () => {
    expect(scopesFor('approval', false)).toEqual(['approval', 'all', 'messaging', 'drive', 'contacts', 'calendar', 'assets'])
    expect(scopesFor('all', false)).toEqual(['all', 'messaging', 'drive', 'contacts', 'calendar', 'approval', 'assets'])
  })

  it('offer no Phê duyệt or Tài sản in a personal workspace, even when opened from one', () => {
    expect(scopesFor('drive', true)).toEqual(['drive', 'all', 'messaging', 'contacts', 'calendar'])
    expect(scopesFor('approval', true)).toEqual(['all', 'messaging', 'drive', 'contacts', 'calendar'])
  })

  it('maps each page to the scope it searches in', () => {
    expect(scopeOfModule('documents')).toBe('drive')
    expect(scopeOfModule('settings')).toBe('all')
    expect(scopeOfModule(undefined)).toBe('all')
    expect(scopeOfModule('assets')).toBe('assets')
  })

  it('group Tất cả by every kind the workspace has, and a module by its own', () => {
    expect(groupsOf('all', false)).toEqual(['actions', 'people', 'conversations', 'messages', 'documents', 'approvals', 'events', 'assets'])
    expect(groupsOf('all', true)).toEqual(['actions', 'people', 'conversations', 'messages', 'documents', 'events'])
    expect(groupsOf('drive', false)).toEqual(['documents'])
    expect(groupsOf('approval', true)).toEqual([])
  })
})

describe('Làm nhanh', () => {
  it('has Tạo đề nghị, Sự kiện mới, Nhắn tin and the settings of the module in an organization', () => {
    expect(labels(quickActions(org({ home: 'approval' })))).toEqual(['Tạo đề nghị', 'Sự kiện mới', 'Nhắn tin', 'Cài đặt Phê duyệt'])
  })

  it('opens the settings page of the module it names, and Cài đặt itself from Tất cả or Người', () => {
    const sectionOf = (home: ActionContext['home']) => quickActions(org({ home })).find((a) => a.kind === 'settings')?.section
    expect(sectionOf('messaging')).toBe('chat')
    expect(sectionOf('calendar')).toBe('calendar')
    expect(sectionOf('drive')).toBe('drive')
    expect(sectionOf('approval')).toBe('approval')
    expect(sectionOf('assets')).toBe('assets')
    expect(sectionOf('all')).toBeUndefined()
    expect(sectionOf('contacts')).toBeUndefined()
  })

  it('has no Tạo đề nghị in a personal workspace', () => {
    expect(labels(quickActions(personal({ home: 'drive' })))).toEqual(['Sự kiện mới', 'Nhắn tin', 'Cài đặt Tài liệu'])
  })

  it('has no Tạo đề nghị for someone with no template to make one from', () => {
    expect(labels(quickActions(org({ canCreateRequest: false })))).not.toContain('Tạo đề nghị')
  })
})

describe('actions that match the words', () => {
  it('offers a command whose words the typing starts, without case or accents', () => {
    expect(labels(typedActions('tao de', org()))).toContain('Tạo đề nghị')
    expect(labels(typedActions('cài đặt lịch', org()))).toEqual(['Cài đặt Lịch'])
    expect(typedActions('cài đặt lịch', org())[0]).toMatchObject({ kind: 'settings', section: 'calendar' })
    expect(labels(typedActions('doi work', org()))).toEqual(['Đổi workspace'])
  })

  it('takes what follows a create command as its subject', () => {
    expect(labels(typedActions('sự kiện mới họp quý', org()))).toEqual(['Sự kiện mới “họp quý”'])
    const [request] = typedActions('Tạo đề nghị mua laptop', org())
    expect(request).toMatchObject({ kind: 'create-request', subject: 'mua laptop', label: 'Tạo đề nghị “mua laptop”' })
  })

  it('offers the create action of the module it was opened in, with the words as its subject', () => {
    expect(labels(typedActions('đối soát', org({ home: 'approval' })))).toEqual(['Tạo đề nghị “đối soát”'])
    expect(labels(typedActions('đối soát', org({ home: 'calendar' })))).toEqual(['Sự kiện mới “đối soát”'])
    expect(typedActions('đối soát', org({ home: 'drive' }))).toEqual([])
  })

  it('offers to message the first person the words name', () => {
    const [a] = typedActions('lan anh', org(), { userId: 'u-lan', name: 'Trần Lan Anh' })
    expect(a).toMatchObject({ kind: 'message-person', label: 'Nhắn tin cho Trần Lan Anh', userId: 'u-lan' })
  })

  it('never offers Tạo đề nghị in a personal workspace, however it is asked for', () => {
    expect(labels(typedActions('tạo đề nghị', personal()))).toEqual([])
    expect(labels(typedActions('tạo đề nghị nghỉ phép', personal({ home: 'approval' })))).toEqual([])
    expect(labels(typedActions('cài đặt phê duyệt', personal()))).toEqual([])
  })

  it('offers nothing for an empty box', () => {
    expect(typedActions('   ', org({ home: 'approval' }))).toEqual([])
  })
})
