import { beforeEach, describe, expect, it } from 'vitest'
import { forgetLastAccount, lastAccount, rememberLastAccount } from './last-account'

beforeEach(() => localStorage.clear())

describe('the last account signed in on this device', () => {
  it('keeps only a name and a picture, never an id or an address', () => {
    rememberLastAccount({
      id: '7f3c2a10-0000-4000-8000-000000000001', email: 'hoa@novapay.vn',
      display_name: 'Lê Thị Hoa', avatar_url: 'https://cdn.test/a.png',
    } as never)
    expect(lastAccount()).toEqual({ name: 'Lê Thị Hoa', avatarUrl: 'https://cdn.test/a.png' })
    const raw = localStorage.getItem('nexus.login.last') ?? ''
    expect(raw).not.toContain('7f3c2a10')
    expect(raw).not.toContain('hoa@novapay.vn')
  })

  it('falls back to the username when there is no display name', () => {
    rememberLastAccount({ display_name: '', username: 'hoa.le', avatar_url: '' })
    expect(lastAccount()).toEqual({ name: 'hoa.le', avatarUrl: '' })
  })

  it('remembers nothing without a name', () => {
    rememberLastAccount({ display_name: '', username: '', avatar_url: '' })
    expect(lastAccount()).toBeNull()
  })

  it('is forgotten on request', () => {
    rememberLastAccount({ display_name: 'Lê Thị Hoa', avatar_url: '' })
    forgetLastAccount()
    expect(lastAccount()).toBeNull()
  })

  it('ignores anything stored that is not its own shape', () => {
    localStorage.setItem('nexus.login.last', '{"name":42}')
    expect(lastAccount()).toBeNull()
    localStorage.setItem('nexus.login.last', 'not json')
    expect(lastAccount()).toBeNull()
  })
})
