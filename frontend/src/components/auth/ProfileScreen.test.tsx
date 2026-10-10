import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../../api/client'
import { ID, calls, apiError, world } from '../../test/auth-fixtures'
import { renderAuthScreen, startAuthTest, where } from '../../test/auth-render'
import { expectNoIds } from '../../test/no-ids'
import { ProfileScreen } from './ProfileScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))

const open = () => renderAuthScreen(ProfileScreen, '/register')
const nameField = () => screen.findByLabelText(/Tên hiển thị/)
const next = () => screen.getByRole('button', { name: 'Tiếp tục' })
const patches = () => calls.filter((c) => c.method === 'PATCH')

beforeEach(() => {
  startAuthTest({ signedIn: true })
  // A brand-new account: its display name is only the handle made from the address.
  world.me.needs_profile = true
  world.me.display_name = world.me.username
})
afterEach(() => vi.restoreAllMocks())

describe('Profile step', () => {
  it('asks one question: what should people call you', async () => {
    await open()
    expect(await screen.findByRole('heading', { level: 1, name: /mọi người sẽ gọi bạn là gì/ })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Bước 2 trên 3' })).toBeInTheDocument()
    expect(await nameField()).toHaveFocus()
  })

  it('shows the person’s initials on their own colour as they type, never the handle', async () => {
    await open()
    const input = await nameField()
    await userEvent.type(input, 'Phạm Thuý An')
    // Vietnamese order: the last two words.
    expect(screen.getByTestId('profile-preview')).toHaveTextContent('TA')
    expect(screen.getByTestId('profile-preview')).not.toHaveTextContent('HO')
  })

  it('starts empty for an account whose name is only its handle', async () => {
    await open()
    expect(await nameField()).toHaveValue('')
  })

  it('starts from the name Google supplied', async () => {
    world.me.display_name = 'Phạm Thuý An'
    await open()
    await waitFor(() => expect(screen.getByLabelText(/Tên hiển thị/)).toHaveValue('Phạm Thuý An'))
  })

  it('offers no photo upload that does nothing', async () => {
    await open()
    await nameField()
    expect(screen.queryByText(/tải ảnh|upload/i)).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).toBeNull()
  })

  it('sends only the name when the title is left empty', async () => {
    await open()
    await userEvent.type(await nameField(), '  Phạm Thuý An  ')
    await userEvent.click(next())
    await waitFor(() => expect(patches()).toHaveLength(1))
    expect(patches()[0]).toEqual({ method: 'PATCH', path: '/me/profile', body: { display_name: 'Phạm Thuý An' } })
  })

  it('sends the title too when given', async () => {
    await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An')
    await userEvent.type(screen.getByLabelText(/Chức danh/), 'Chuyên viên đối soát')
    await userEvent.click(next())
    await waitFor(() => expect(patches()).toHaveLength(1))
    expect(patches()[0]?.body).toEqual({ display_name: 'Phạm Thuý An', title: 'Chuyên viên đối soát' })
  })

  it('goes on to choose a workspace once saved', async () => {
    const router = await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An{Enter}')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
  })

  it('carries the workspace a link named on to the list', async () => {
    const router = await renderAuthScreen(ProfileScreen, '/register', `/register?ws=${ID.wsProject}`)
    await userEvent.type(await nameField(), 'Phạm Thuý An{Enter}')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(where(router).search).toEqual({ ws: ID.wsProject })
  })

  it('a person with nothing owed is moved on with it too', async () => {
    world.me.needs_profile = false
    const router = await renderAuthScreen(ProfileScreen, '/register', `/register?ws=${ID.wsProject}`)
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(where(router).search).toEqual({ ws: ID.wsProject })
  })

  it('does not accept an empty name, and says what to do', async () => {
    await open()
    await nameField()
    await userEvent.click(next())
    expect(await screen.findByText(/Nhập tên/)).toBeInTheDocument()
    expect(patches()).toHaveLength(0)
    expect(await nameField()).toHaveAttribute('aria-invalid', 'true')
  })

  it('does not accept a name of only spaces', async () => {
    await open()
    await userEvent.type(await nameField(), '    {Enter}')
    expect(await screen.findByText(/Nhập tên/)).toBeInTheDocument()
    expect(patches()).toHaveLength(0)
  })

  it('stops at 80 characters, as the server does, and shows the count', async () => {
    await open()
    const input = await nameField()
    expect(input).toHaveAttribute('maxlength', '80')
    await userEvent.type(input, 'a'.repeat(90))
    expect(input).toHaveValue('a'.repeat(80))
    expect(screen.getByText('80/80')).toBeInTheDocument()
  })

  it('shows the server’s refusal under the field', async () => {
    world.profileError = apiError(400, { code: 'invalid_input' })
    await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An{Enter}')
    expect(await screen.findByText(/Tên chưa hợp lệ/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/invalid_input|server words/)
  })

  it.each([
    ['a server fault', apiError(500, { code: 'internal' }), /Máy chủ đang gặp sự cố/],
    ['no network', new TypeError('Failed to fetch'), /Không kết nối được/],
  ])('shows %s as an alert and lets the person try again', async (_n, error, text) => {
    world.profileError = error
    const router = await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent(text)
    expect(where(router).path).toBe('/register')
    world.profileError = undefined
    await userEvent.click(next())
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
  })

  it('does not let a second tap send the profile twice', async () => {
    await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An')
    await userEvent.dblClick(next())
    await waitFor(() => expect(patches().length).toBeGreaterThan(0))
    expect(patches()).toHaveLength(1)
  })

  it('has nothing to ask a person who has already done this, and moves on', async () => {
    world.me.needs_profile = false
    const router = await open()
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(patches()).toHaveLength(0)
  })

  it('shows the shape of the form while the account loads, then the form', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      if (path === '/me') await gate
      return original(path, opts)
    })
    await open()
    expect(document.querySelector('[data-skeleton]')).not.toBeNull()
    release()
    expect(await nameField()).toBeInTheDocument()
    expect(document.querySelector('[data-skeleton]')).toBeNull()
  })

  it('says so, with a retry, when the account cannot be loaded', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    let fail = true
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      if (path === '/me' && fail) throw apiError(500)
      return original(path, opts)
    })
    await open()
    expect(await screen.findByText(/Không tải được/)).toBeInTheDocument()
    fail = false
    await userEvent.click(screen.getByRole('button', { name: /Thử lại/ }))
    expect(await nameField()).toBeInTheDocument()
  })

  it('shows no identifier', async () => {
    await open()
    await userEvent.type(await nameField(), 'Phạm Thuý An')
    expectNoIds()
  })
})
