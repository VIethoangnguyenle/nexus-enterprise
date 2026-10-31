import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../../api/client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import { SettingsLayoutContext, type SettingsLayout } from './settings-layout'
import { WS, renderSettings, resetSettingsServer, server, settingsAnswer, viewport } from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  resetSettingsServer()
  api.mockReset()
  api.mockImplementation(settingsAnswer as typeof apiFetch)
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

const ORG_ITEMS = [
  'Tài khoản và hồ sơ', 'Thông báo', 'Giao diện, ngôn ngữ', 'Thiết bị và phiên',
  'Hồ sơ trong tổ chức', 'Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Tài sản', 'Cài đặt workspace',
]
const SECTION_OF: Record<string, string> = {
  'Tài khoản và hồ sơ': 'account', 'Thông báo': 'notifications', 'Giao diện, ngôn ngữ': 'appearance',
  'Thiết bị và phiên': 'sessions', 'Hồ sơ trong tổ chức': 'org-profile', Chat: 'chat', Lịch: 'calendar',
  'Tài liệu': 'drive', 'Phê duyệt': 'approval', 'Tài sản': 'assets', 'Cài đặt workspace': 'workspace',
}

const nav = () => screen.findByRole('navigation', { name: 'Mục cài đặt' })
const links = (el: HTMLElement) => within(el).getAllByRole('link').map((a) => a.textContent)

describe('Cài đặt on a wide screen', () => {
  beforeEach(() => viewport(false))

  it('opens on Tài khoản và hồ sơ beside the section list, both groups in the order of Tôi', async () => {
    await renderSettings(`/settings?ws=${WS}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Cài đặt' })).toBeInTheDocument()
    const list = await nav()
    await waitFor(() => expect(links(list)).toEqual(ORG_ITEMS))
    expect(within(list).getByText('Tài khoản')).toBeInTheDocument()
    expect(await within(list).findByText('Trong NovaPay')).toBeInTheDocument()
    expect(within(list).getByRole('link', { name: 'Tài khoản và hồ sơ' })).toHaveAttribute('aria-current', 'page')
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài khoản và hồ sơ' })).toBeInTheDocument()
    // No phone furniture: the page has no back button and the section list replaces Tôi.
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expect(screen.queryByRole('heading', { name: 'Tôi' })).toBeNull()
    expect(screen.queryByRole('tab')).toBeNull()
    expectNoIds()
  })

  it('in a personal workspace lists no Hồ sơ trong tổ chức, Phê duyệt or Tài sản', async () => {
    server.personal = true
    await renderSettings(`/settings?ws=${WS}`)
    const list = await nav()
    await waitFor(() => expect(links(list)).toEqual(
      ['Tài khoản và hồ sơ', 'Thông báo', 'Giao diện, ngôn ngữ', 'Thiết bị và phiên', 'Chat', 'Lịch', 'Tài liệu', 'Cài đặt workspace'],
    ))
    expect(within(list).getByText('Trong workspace này')).toBeInTheDocument()
  })

  it('makes every item a link to its own address that keeps the workspace', async () => {
    await renderSettings(`/settings?ws=${WS}`)
    const list = await nav()
    await waitFor(() => expect(links(list)).toEqual(ORG_ITEMS))
    for (const a of within(list).getAllByRole('link')) {
      expect(a).toHaveAttribute('href', `/settings/${SECTION_OF[a.textContent ?? '']}?ws=${WS}`)
    }
  })

  it('follows a link: the address changes, the section opens and becomes the current item', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings?ws=${WS}`)
    await user.click(within(await nav()).getByRole('link', { name: 'Thiết bị và phiên' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/sessions'))
    expect(router.state.location.search).toEqual({ ws: WS })
    expect(await screen.findByRole('heading', { level: 1, name: 'Thiết bị và phiên' })).toBeInTheDocument()
    const list = await nav()
    expect(within(list).getByRole('link', { name: 'Thiết bị và phiên' })).toHaveAttribute('aria-current', 'page')
    expect(within(list).getByRole('link', { name: 'Tài khoản và hồ sơ' })).not.toHaveAttribute('aria-current')
  })

  it('opens a section from its address inside the shell, without a back button', async () => {
    await renderSettings(`/settings/sessions?ws=${WS}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Thiết bị và phiên' })).toBeInTheDocument()
    expect(within(await nav()).getByRole('link', { name: 'Thiết bị và phiên' })).toHaveAttribute('aria-current', 'page')
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
  })

  it.each([
    ['giao-dien', 'appearance'],
    ['workspace', 'workspace'],
    ['thong-bao', 'notifications'],
  ])('sends an old ?tab=%s link to /settings/%s, keeping the workspace', async (tab, section) => {
    const router = await renderSettings(`/settings?ws=${WS}&tab=${tab}`)
    await waitFor(() => expect(router.state.location.pathname).toBe(`/settings/${section}`))
    expect(router.state.location.search).toEqual({ ws: WS })
    expect(within(await nav()).getByRole('link', { name: { appearance: 'Giao diện, ngôn ngữ', workspace: 'Cài đặt workspace', notifications: 'Thông báo' }[section] }))
      .toHaveAttribute('aria-current', 'page')
  })

  it.each(['ho-so', 'nope'])('opens Tài khoản và hồ sơ for the old ?tab=%s', async (tab) => {
    const router = await renderSettings(`/settings?ws=${WS}&tab=${tab}`)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/account'))
    expect(router.state.location.search).toEqual({ ws: WS })
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài khoản và hồ sơ' })).toBeInTheDocument()
  })



  it('lands a personal workspace on Tài khoản và hồ sơ for a page only an organization has', async () => {
    server.personal = true
    const router = await renderSettings(`/settings/approval?ws=${WS}`)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/account'))
    expect(router.state.location.search).toEqual({ ws: WS })
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài khoản và hồ sơ' })).toBeInTheDocument()
  })

  it('ends the list with Giới thiệu and the build the person is running', async () => {
    vi.stubEnv('VITE_APP_VERSION', '2026.10.18 · 6368e8e')
    await renderSettings(`/settings?ws=${WS}`)
    const list = await nav()
    const line = within(list).getByText('Giới thiệu · 2026.10.18 · 6368e8e')
    expect(list.lastElementChild).toBe(line)
  })
})

describe('Cài đặt on a phone stays as it is', () => {
  beforeEach(() => viewport(true))

  it('/settings is Tôi, with no section list', async () => {
    await renderSettings(`/settings?ws=${WS}`)
    expect(await screen.findByRole('heading', { name: 'Tôi' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Mục cài đặt' })).toBeNull()
  })

  it('a section page fills the screen and has its back button', async () => {
    await renderSettings(`/settings/sessions?ws=${WS}`)
    expect(await screen.findByRole('heading', { name: 'Thiết bị và phiên' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Quay lại' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Mục cài đặt' })).toBeNull()
  })
})

describe('The settings pieces in each layout', () => {
  const page = (layout?: SettingsLayout) => {
    const onBack = vi.fn()
    const tree = (
      <SettingsPage title="Chat" lead="Trong NovaPay" onBack={onBack}>
        <SettingsGroup title="Thông báo"><SettingsRow label="Được nhắc tên" hint="Kể cả @Tất cả" /></SettingsGroup>
      </SettingsPage>
    )
    render(layout ? <SettingsLayoutContext value={layout}>{tree}</SettingsLayoutContext> : tree)
    return onBack
  }

  it('wide: the title and its lead line, no back button, the group and its row', () => {
    page('wide')
    expect(screen.getByRole('heading', { level: 1, name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByText('Trong NovaPay')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expect(screen.getByRole('heading', { level: 2, name: 'Thông báo' })).toBeInTheDocument()
    expect(screen.getByText('Được nhắc tên')).toBeInTheDocument()
    expect(screen.getByText('Kể cả @Tất cả')).toBeInTheDocument()
  })

  it('phone, the default: the back bar and no lead line', async () => {
    const onBack = page()
    expect(screen.getByRole('heading', { level: 1, name: 'Chat' })).toBeInTheDocument()
    expect(screen.queryByText('Trong NovaPay')).toBeNull()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Quay lại' }))
    expect(onBack).toHaveBeenCalledOnce()
  })
})
