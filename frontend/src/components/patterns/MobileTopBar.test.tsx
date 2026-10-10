import '@testing-library/jest-dom/vitest'
import { useState } from 'react'
import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { Button, SearchField } from '../primitives'
import { MobileTopBar } from './MobileTopBar'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

/** The shell's arrangement: the bar above the screen it watches. */
function Shell({ screen: body }: { screen: React.ReactNode }) {
  const [el, setEl] = useState<HTMLElement | null>(null)
  return (
    <>
      <MobileTopBar scope={el} />
      <div ref={setEl}>{body}</div>
    </>
  )
}

beforeEach(() => {
  viewport(true)
  vi.mocked(apiFetch).mockImplementation((path: string) =>
    path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] }) : Promise.reject(new Error(path)))
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('MobileTopBar', () => {
  it('names the product and the workspace, and has no Menu button', async () => {
    renderWithClient(<Shell screen={<h1>Phê duyệt</h1>} />)
    expect(await screen.findByText(/Khối Vận hành/)).toHaveTextContent('Nexus · Khối Vận hành')
    expect(screen.queryByRole('button', { name: /menu/i })).toBeNull()
    expect(screen.getByRole('heading', { name: 'Phê duyệt' })).toBeInTheDocument()
  })

  it('has no search action on a screen without a search of its own', async () => {
    renderWithClient(<Shell screen={<SearchField label="Tìm vai trò" />} />)
    await screen.findByText(/Khối Vận hành/)
    expect(screen.queryByRole('button', { name: 'Tìm kiếm' })).toBeNull()
  })

  it('takes focus to the screen\'s own search box', async () => {
    const user = userEvent.setup()
    renderWithClient(<Shell screen={<SearchField label="Tìm tài sản" moduleSearch />} />)
    await user.click(await screen.findByRole('button', { name: 'Tìm kiếm' }))
    expect(screen.getByRole('searchbox', { name: 'Tìm tài sản' })).toHaveFocus()
  })

  it('presses the search button of a screen whose search is a panel', async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    renderWithClient(<Shell screen={<Button data-module-search onClick={open}>Tìm trong nhóm</Button>} />)
    await user.click(await screen.findByRole('button', { name: 'Tìm kiếm' }))
    expect(open).toHaveBeenCalledTimes(1)
  })

  it('follows the screen: the action appears when its search does and goes when it does', async () => {
    const { rerender } = renderWithClient(<Shell screen={<p>Trang chủ</p>} />)
    await screen.findByText(/Khối Vận hành/)
    expect(screen.queryByRole('button', { name: 'Tìm kiếm' })).toBeNull()
    rerender(<Shell screen={<SearchField label="Tìm tài sản" moduleSearch />} />)
    await screen.findByRole('button', { name: 'Tìm kiếm' })
    rerender(<Shell screen={<p>Trang chủ</p>} />)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Tìm kiếm' })).toBeNull())
  })

  it('is not drawn at desktop widths', async () => {
    viewport(false)
    renderWithClient(<Shell screen={<p>x</p>} />)
    expect(screen.queryByRole('banner')).toBeNull()
  })
})
