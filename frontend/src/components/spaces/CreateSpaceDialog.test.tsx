import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { U, WS_ID } from '../../test/chat-fixtures'
import { useToastStore } from '../primitives'
import { ApiError } from '../../api/client'
import { CreateSpaceDialog } from './CreateSpaceDialog'

const NEW_ID = '99999999-aaaa-4bbb-8ccc-000000000001'
const calls: { path: string; method: string; body?: unknown }[] = []
let createFails: number | null = null

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const actual = await orig<typeof import('../../api/client')>()
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return {
    ...actual,
    apiFetch: vi.fn((path: string, init?: RequestInit) => {
      const method = (init?.method || 'GET').toUpperCase()
      if (method !== 'GET') calls.push({ path, method, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (path === `/workspaces/${WS_ID}/channels` && method === 'POST') {
        if (createFails) return Promise.reject(new actual.ApiError('forbidden', createFails))
        return Promise.resolve({ id: NEW_ID, name: 'Đối soát tháng 10', channel_type: 'workspace', workspace_id: WS_ID })
      }
      if (path === `/channels/${NEW_ID}/members` && method === 'POST') return Promise.resolve({ status: 'ok' })
      return fixtureApi(path, init)
    }),
  }
})

beforeEach(() => {
  calls.length = 0
  createFails = null
  routerState.navigate.mockReset()
  useToastStore.getState().clear()
})
afterEach(resetClient)

async function pick(user: ReturnType<typeof userEvent.setup>, query: string, name: string) {
  const picker = screen.getByRole('combobox', { name: 'Thêm người' })
  await user.click(picker)
  await user.type(picker, query)
  const option = await screen.findByRole('option', { name: new RegExp(name) })
  await user.click(option)
}

describe('CreateSpaceDialog', () => {
  it('creates the space with its name, adds the picked people, opens it and confirms', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    renderWithClient(<CreateSpaceDialog open onClose={onClose} />)

    await user.type(screen.getByRole('textbox', { name: 'Tên nhóm' }), 'Đối soát tháng 10')
    expect(screen.getByText('17/128')).toBeInTheDocument()

    await pick(user, 'duc', 'Trần Minh Đức')
    await pick(user, 'Lan', 'Nguyễn Thu Lan')
    // Picked people show as chips, by name.
    expect(screen.getByRole('button', { name: 'Bỏ Trần Minh Đức' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bỏ Nguyễn Thu Lan' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Tạo nhóm' }))

    await waitFor(() => expect(routerState.navigate).toHaveBeenCalled())
    expect(calls[0]).toEqual({
      path: `/workspaces/${WS_ID}/channels`,
      method: 'POST',
      body: { name: 'Đối soát tháng 10', channel_type: 'workspace' },
    })
    const added = calls.filter((c) => c.path === `/channels/${NEW_ID}/members`).map((c) => c.body)
    expect(added).toEqual(
      expect.arrayContaining([{ ngac_node_id: U.duc.node }, { ngac_node_id: U.lan.node }]),
    )
    expect(added).toHaveLength(2)
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: NEW_ID } })
    expect(onClose).toHaveBeenCalled()
    expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã tạo nhóm “Đối soát tháng 10”')
  })

  it('has no free-text identifier field: people are only added by searching names', () => {
    renderWithClient(<CreateSpaceDialog open onClose={vi.fn()} />)
    const dialog = screen.getByRole('dialog', { name: 'Tạo nhóm' })
    const fields = within(dialog).queryAllByRole('textbox').concat(within(dialog).queryAllByRole('combobox'))
    expect(fields).toHaveLength(2)
    for (const f of fields) {
      const label = [f.getAttribute('aria-label'), f.getAttribute('placeholder'), f.getAttribute('name'), f.id]
        .join(' ')
      expect(label).not.toMatch(/\b(id|uuid|mã|node)\b/i)
    }
    expect(within(dialog).getByRole('combobox', { name: 'Thêm người' })).toBeInTheDocument()
  })

  it('does not offer yourself, and hides people already picked', async () => {
    const user = userEvent.setup()
    renderWithClient(<CreateSpaceDialog open onClose={vi.fn()} />)
    const picker = screen.getByRole('combobox', { name: 'Thêm người' })
    await user.type(picker, 'Lê')
    const list = await screen.findByRole('listbox')
    expect(within(list).queryByText('Lê Thị Hoa')).toBeNull()
    await user.click(within(list).getByRole('option', { name: /Lê Quang Vinh/ }))
    await user.type(picker, 'Lê')
    const again = await screen.findByRole('listbox')
    expect(within(again).queryByRole('option', { name: /Lê Quang Vinh/ })).toBeNull()
  })

  it('asks for a name before creating', async () => {
    const user = userEvent.setup()
    renderWithClient(<CreateSpaceDialog open onClose={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: 'Tạo nhóm' }))
    expect(await screen.findByText('Đặt tên cho nhóm')).toBeInTheDocument()
    expect(calls).toHaveLength(0)
  })

  it('explains a missing permission instead of failing silently', async () => {
    createFails = 403
    const user = userEvent.setup()
    const onClose = vi.fn()
    renderWithClient(<CreateSpaceDialog open onClose={onClose} />)
    await user.type(screen.getByRole('textbox', { name: 'Tên nhóm' }), 'Kiểm soát')
    await user.click(screen.getByRole('button', { name: 'Tạo nhóm' }))
    await waitFor(() => expect(useToastStore.getState().toasts).toHaveLength(1))
    const t = useToastStore.getState().toasts[0]!
    expect(t.tone).toBe('error')
    expect(t.message).toMatch(/quyền tạo nhóm/)
    expect(onClose).not.toHaveBeenCalled()
    expect(ApiError).toBeDefined()
  })
})
