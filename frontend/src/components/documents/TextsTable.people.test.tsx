import { cleanup, fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import type { TextDocument } from '../../api/documents'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { renderWithClient, resetClient } from '../../test/render'
import { TextsTable } from './TextsTable'

const HIDDEN = '11111111-aaaa-4bbb-8ccc-0000000000ff'
const people = buildDirectory(CONTACTS, {
  resolved: {
    byUserId: new Map([[HIDDEN, { userId: HIDDEN, nodeId: 'n-hidden', name: 'Nguyễn Hữu Thành', avatarUrl: '' }]]),
    byNodeId: new Map(),
  },
})

const doc = (id: string, owner_id: string, owner_name: string): TextDocument => ({
  id, workspace_id: 'w', title: 'Biên bản đối soát', version: 1, status: 'draft', owner_id, owner_name,
  created_at: '2026-10-09T01:30:00Z', updated_at: '2026-10-09T01:30:00Z',
})

async function renderTable(docs: TextDocument[]) {
  const root = createRootRoute()
  const list = createRoute({
    getParentRoute: () => root, path: '/',
    component: () => <TextsTable label="Văn bản" docs={docs} loading={false} people={people} fresh={new Map()} scope="s" onDelete={() => {}} />,
  })
  const editor = createRoute({ getParentRoute: () => root, path: '/documents/$docId', component: () => <p>Trình soạn thảo</p> })
  const router = createRouter({ routeTree: root.addChildren([list, editor]), history: createMemoryHistory({ initialEntries: ['/'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => { vi.spyOn(window, 'scrollTo').mockImplementation(() => {}) })
afterEach(() => { cleanup(); vi.restoreAllMocks(); resetClient() })

describe('TextsTable: pressing the owner', () => {
  it('a hidden owner keeps their name and opens the small card, not the document', async () => {
    const router = await renderTable([doc('d1', HIDDEN, '')])
    await userEvent.click(await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    expect(within(card).getByText('Hồ sơ này không còn hiển thị với bạn.')).toBeInTheDocument()
    expect(within(card).queryAllByRole('button')).toHaveLength(0)
    expect(router.state.location.pathname).toBe('/')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('a visible owner opens their profile in Danh bạ rather than the document', async () => {
    const router = await renderTable([doc('d2', U.duc.id, 'Trần Minh Đức')])
    await userEvent.click(await screen.findByRole('button', { name: 'Trần Minh Đức' }))
    expect(router.state.location.pathname).not.toBe('/documents/d2')
  })

  it('a right-click in the card does not open the row menu', async () => {
    await renderTable([doc('d3', HIDDEN, '')])
    await userEvent.click(await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' }))
    const card = await screen.findByRole('dialog', { name: 'Nguyễn Hữu Thành' })
    fireEvent.contextMenu(card)
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('nor does a right-click on the owner name', async () => {
    await renderTable([doc('d4', HIDDEN, '')])
    fireEvent.contextMenu(await screen.findByRole('button', { name: 'Nguyễn Hữu Thành' }))
    expect(screen.queryByRole('menu')).toBeNull()
  })
})
