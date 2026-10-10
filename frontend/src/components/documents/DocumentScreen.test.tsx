import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import type { Editor } from '@tiptap/react'
import { renderWithClient, resetClient } from '../../test/render'
import { DOCS, T, calls, docsFixtureApi, hooks, resetDocs } from '../../test/documents-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { ApiError, apiFetch } from '../../api/client'
import { validateDriveSearch } from '../../lib/drive-search'
import { useToastStore } from '../primitives'
import { DocumentScreen } from './DocumentScreen'

// ProseMirror measures ranges when it scrolls the caret into view; jsdom has no layout.
const noRects = { length: 0, item: () => null, [Symbol.iterator]: function* () {} } as unknown as DOMRectList
Range.prototype.getClientRects = () => noRects
Range.prototype.getBoundingClientRect = () => new DOMRect()

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

async function renderDoc(id: string) {
  const root = createRootRoute()
  const doc = createRoute({ getParentRoute: () => root, path: '/documents/$docId', component: DocumentScreen })
  const list = createRoute({ getParentRoute: () => root, path: '/drive', validateSearch: validateDriveSearch, component: () => <p>Danh sách văn bản</p> })
  const router = createRouter({
    routeTree: root.addChildren([doc, list]),
    history: createMemoryHistory({ initialEntries: [`/documents/${id}`] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const prose = () => document.querySelector('.doc-prose') as HTMLElement & { editor: Editor }
const editor = () => prose().editor
/** Types into the page the way the editor reports it: through the editor, one change. */
const write = (html: string) => act(() => { editor().commands.setContent(html, { emitUpdate: true }) })
const patches = () => calls.filter((c) => c.method === 'PATCH')

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetDocs()
  api.mockImplementation(docsFixtureApi)
})
afterEach(() => {
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

async function open(id = T.quytrinh) {
  const router = await renderDoc(id)
  await waitFor(() => expect(document.querySelector('.doc-prose')).not.toBeNull())
  return router
}

describe('DocumentScreen: opening', () => {
  it('shows the title, the text, who owns it and that it is saved, with no id', async () => {
    await open()
    expect(screen.getByRole('textbox', { name: 'Tiêu đề văn bản' })).toHaveValue('Quy trình đối soát cuối ngày')
    expect(prose()).toHaveTextContent('1. Chuẩn bị số liệu')
    expect(prose()).toHaveTextContent('Đặt vào thư mục Đối soát.')
    expect(screen.getByText('Lê Thị Hoa')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Đã lưu')
    expect(screen.getByRole('button', { name: 'Trạng thái: Bản nháp' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Lưu/ })).toBeNull() // no Save button
    expectNoIds()
  })

  it('draws no cursors or presence: this is a one-author editor', async () => {
    await open()
    expect(screen.queryByText(/đang cùng sửa/i)).toBeNull()
    expect(document.querySelector('[class*="caret"]')).toBeNull()
  })

  it('shows a skeleton while the document loads', async () => {
    api.mockImplementation((path, init) =>
      path.startsWith('/documents/texts/') ? new Promise(() => {}) : docsFixtureApi(path, init))
    await renderDoc(T.quytrinh)
    await waitFor(() => expect(document.querySelector('[aria-busy="true"]')).not.toBeNull())
  })

  it('says a document cannot be opened, and points back to the list', async () => {
    const router = await renderDoc('99999999-aaaa-4bbb-8ccc-0000000000aa')
    expect(await screen.findByText(/Không mở được văn bản này/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: 'Về danh sách văn bản' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/drive'))
    expect(router.state.location.search).toMatchObject({ view: 'texts' })
  })

  it('says it could not load, and retries', async () => {
    hooks.failNext = { scope: 'document', make: () => Promise.reject(new ApiError('boom', 500)) }
    await renderDoc(T.quytrinh)
    expect(await screen.findByText(/Không tải được văn bản/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Thử lại' }))
    await waitFor(() => expect(document.querySelector('.doc-prose')).not.toBeNull())
  })

  it('is read-only for someone who may only read: no toolbar, no title field, said in words', async () => {
    await open(T.sla)
    expect(screen.getByRole('heading', { level: 1, name: 'Cam kết SLA với ngân hàng đối tác' })).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: 'Tiêu đề văn bản' })).toBeNull()
    expect(screen.queryByRole('toolbar')).toBeNull()
    expect(prose()).toHaveAttribute('contenteditable', 'false')
    expect(screen.getByText('Bạn chỉ có quyền xem văn bản này.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thao tác với văn bản' })).toBeNull()
  })

  it('never lets stored markup run: scripts and handlers are dropped before the editor sees them', async () => {
    DOCS[T.quytrinh]!.content = '<p>an toàn</p><script>window.__pwned = 1</script><img src="x" onerror="window.__pwned = 2"><p onclick="window.__pwned = 3">bấm</p>'
    await open()
    expect(prose()).toHaveTextContent('an toàn')
    expect(prose().innerHTML).not.toMatch(/<script|onerror|onclick/i)
    expect((window as unknown as { __pwned?: number }).__pwned).toBeUndefined()
  })
})

describe('DocumentScreen: saving', () => {
  it('saves what is typed on its own, once the typing pauses, and says when', async () => {
    await open()
    write('<p>Đã sửa một dòng</p>')
    expect(screen.getByRole('status')).toHaveTextContent('Chưa lưu')

    await waitFor(() => expect(patches()).toHaveLength(1), { timeout: 4000 })
    expect(patches()[0]!.body).toEqual({ base_version: 1, content: '<p>Đã sửa một dòng</p>' })
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent(/Đã lưu \d\d:\d\d/))
    expect(DOCS[T.quytrinh]!.version).toBe(2)
  })

  it('saves at once with Ctrl+S', async () => {
    const user = userEvent.setup()
    await open()
    write('<p>ngay</p>')
    prose().focus()
    await user.keyboard('{Control>}s{/Control}')
    await waitFor(() => expect(patches()).toHaveLength(1))
  })

  it('saves a new title with the version it was based on', async () => {
    const user = userEvent.setup()
    await open()
    const title = screen.getByRole('textbox', { name: 'Tiêu đề văn bản' })
    await user.clear(title)
    await user.type(title, 'Quy trình mới')
    await user.keyboard('{Control>}s{/Control}')
    await waitFor(() => expect(patches()).toHaveLength(1))
    expect(patches()[0]!.body).toEqual({ base_version: 1, title: 'Quy trình mới' })
    expect(screen.getByRole('navigation', { name: 'Đường dẫn' })).toHaveTextContent('Quy trình mới')
  })

  it('does not leave a document without a title', async () => {
    const user = userEvent.setup()
    await open()
    const title = screen.getByRole('textbox', { name: 'Tiêu đề văn bản' })
    await user.clear(title)
    await user.tab()
    expect(title).toHaveValue('Quy trình đối soát cuối ngày')
  })

  it('applies the toolbar to the selection', async () => {
    const user = userEvent.setup()
    await open()
    write('<p>chữ</p>')
    act(() => { editor().commands.selectAll() })
    await user.click(screen.getByRole('button', { name: 'In đậm' }))
    expect(editor().getHTML()).toBe('<p><strong>chữ</strong></p>')
    expect(screen.getByRole('button', { name: 'In đậm' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('changes the paragraph style from the menu', async () => {
    const user = userEvent.setup()
    await open()
    write('<p>Một dòng</p>')
    act(() => { editor().commands.setTextSelection(2) }) // the caret is in that one line
    await user.click(screen.getByRole('button', { name: /Kiểu đoạn: Đoạn văn/ }))
    await user.click(await screen.findByRole('menuitem', { name: 'Tiêu đề lớn' }))
    expect(editor().getHTML()).toContain('<h1>Một dòng</h1>')
    expect(await screen.findByRole('button', { name: /Kiểu đoạn: Tiêu đề lớn/ })).toBeInTheDocument()
  })

  it('changes the status from the menu, saved at once', async () => {
    const user = userEvent.setup()
    await open()
    await user.click(screen.getByRole('button', { name: 'Trạng thái: Bản nháp' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đang dùng' }))
    await waitFor(() => expect(patches()).toHaveLength(1))
    expect(patches()[0]!.body).toEqual({ base_version: 1, status: 'active' })
    expect(await screen.findByRole('button', { name: 'Trạng thái: Đang dùng' })).toBeInTheDocument()
  })

  it('says it is waiting for the network when a save cannot reach the server', async () => {
    const user = userEvent.setup()
    await open()
    let first = true
    api.mockImplementation((path, init) => {
      if (init?.method === 'PATCH' && first) { first = false; return Promise.reject(new TypeError('Failed to fetch')) }
      return docsFixtureApi(path, init)
    })
    write('<p>mất mạng</p>')
    prose().focus()
    await user.keyboard('{Control>}s{/Control}')

    expect(await screen.findByText('Chưa lưu, đang chờ mạng')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent(/sẽ tự lưu khi có mạng/)
    expect(prose()).toHaveTextContent('mất mạng') // nothing typed is lost
  })
})

describe('DocumentScreen: someone saved first', () => {
  /** Another person saves between this one's load and save. */
  const someoneElseSaves = () => {
    hooks.beforeSave = (id) => {
      DOCS[id]!.content = '<h2>1. Chuẩn bị số liệu</h2><p>Bản của người khác.</p>'
      DOCS[id]!.title = 'Quy trình đối soát cuối ngày'
      DOCS[id]!.version = 4
      hooks.beforeSave = undefined
    }
  }

  async function conflicted() {
    const user = userEvent.setup()
    await open()
    someoneElseSaves()
    write('<h2>1. Chuẩn bị số liệu</h2><p>Bản của tôi.</p>')
    prose().focus()
    await user.keyboard('{Control>}s{/Control}')
    const dialog = await screen.findByRole('dialog', { name: 'Văn bản đã được sửa ở nơi khác' })
    return { user, dialog }
  }

  it('asks which version to keep, and has overwritten nothing', async () => {
    const { dialog } = await conflicted()
    expect(within(dialog).getByRole('button', { name: 'Tải lại bản mới nhất' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Giữ bản của tôi' })).toBeInTheDocument()
    expect(DOCS[T.quytrinh]!.content).toContain('Bản của người khác.')
    expect(DOCS[T.quytrinh]!.version).toBe(4)
    expect(patches()).toHaveLength(1)
    expect(screen.getByRole('status')).toHaveTextContent('Chưa lưu: có bản mới hơn')
  })

  it('does not retry or overwrite while the choice is open, however long', async () => {
    await conflicted()
    await new Promise((r) => setTimeout(r, 1800))
    expect(patches()).toHaveLength(1)
    expect(DOCS[T.quytrinh]!.content).toContain('Bản của người khác.')
  })

  it('compares the two versions side by side, marking the lines that differ', async () => {
    const { user, dialog } = await conflicted()
    await user.click(within(dialog).getByRole('button', { name: 'So sánh' }))

    const mine = within(dialog).getByRole('region', { name: 'Bản của bạn' })
    const theirs = within(dialog).getByRole('region', { name: 'Bản trên máy chủ' })
    expect(within(mine).getByText('Bản của tôi.')).toHaveAttribute('data-changed')
    expect(within(theirs).getByText('Bản của người khác.')).toHaveAttribute('data-changed')
    expect(within(mine).getByText('1. Chuẩn bị số liệu')).not.toHaveAttribute('data-changed')
    expect(patches()).toHaveLength(1) // looking writes nothing
  })

  it('"Tải lại bản mới nhất" puts the other version on the page and drops mine', async () => {
    const { user, dialog } = await conflicted()
    await user.click(within(dialog).getByRole('button', { name: 'Tải lại bản mới nhất' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(prose()).toHaveTextContent('Bản của người khác.')
    expect(prose()).not.toHaveTextContent('Bản của tôi.')
    expect(screen.getByRole('status')).toHaveTextContent('Đã lưu')
    expect(patches()).toHaveLength(1)

    // Writing on from there builds on their version.
    write('<p>tiếp theo</p>')
    prose().focus()
    await user.keyboard('{Control>}s{/Control}')
    await waitFor(() => expect(patches()).toHaveLength(2))
    expect(patches()[1]!.body).toEqual({ base_version: 4, content: '<p>tiếp theo</p>' })
  })

  it('"Giữ bản của tôi" saves mine on top of theirs, because it was asked for', async () => {
    const { user, dialog } = await conflicted()
    await user.click(within(dialog).getByRole('button', { name: 'Giữ bản của tôi' }))

    await waitFor(() => expect(patches()).toHaveLength(2))
    expect(patches()[1]!.body).toEqual({ base_version: 4, content: '<h2>1. Chuẩn bị số liệu</h2><p>Bản của tôi.</p>' })
    await waitFor(() => expect(DOCS[T.quytrinh]!.content).toContain('Bản của tôi.'))
    expect(DOCS[T.quytrinh]!.version).toBe(5)
  })

  it('closing the dialog decides nothing; a banner brings it back', async () => {
    const { user } = await conflicted()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    expect(patches()).toHaveLength(1)
    expect(prose()).toHaveTextContent('Bản của tôi.') // my text is still on the page
    await user.click(screen.getByRole('button', { name: 'Chọn bản giữ lại' }))
    expect(await screen.findByRole('dialog', { name: 'Văn bản đã được sửa ở nơi khác' })).toBeInTheDocument()
  })

  it('shows no id in the dialog either', async () => {
    const { user, dialog } = await conflicted()
    await user.click(within(dialog).getByRole('button', { name: 'So sánh' }))
    expectNoIds()
  })
})

describe('DocumentScreen: deleting', () => {
  it('keeps saving after a delete that was refused', async () => {
    const user = userEvent.setup()
    await open()
    api.mockImplementation((path, init) =>
      init?.method === 'DELETE' ? Promise.reject(new ApiError('boom', 500)) : docsFixtureApi(path, init))
    await user.click(screen.getByRole('button', { name: 'Thao tác với văn bản' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Xoá văn bản' }))
    await user.click(within(await screen.findByRole('dialog', { name: 'Xoá văn bản' })).getByRole('button', { name: 'Xoá' }))
    await waitFor(() => expect(DOCS[T.quytrinh]).toBeDefined())

    write('<p>vẫn viết tiếp</p>')
    prose().focus()
    await user.keyboard('{Control>}s{/Control}')
    await waitFor(() => expect(patches()).toHaveLength(1))
    expect(DOCS[T.quytrinh]!.content).toBe('<p>vẫn viết tiếp</p>')
  })

  it('asks first, then removes the document and goes back to the list', async () => {
    const user = userEvent.setup()
    const router = await open()
    await user.click(screen.getByRole('button', { name: 'Thao tác với văn bản' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Xoá văn bản' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá văn bản' })
    expect(DOCS[T.quytrinh]).toBeDefined()

    await user.click(within(dialog).getByRole('button', { name: 'Xoá' }))
    await waitFor(() => expect(DOCS[T.quytrinh]).toBeUndefined())
    await waitFor(() => expect(router.state.location.pathname).toBe('/drive'))
    expect(patches()).toHaveLength(0) // leaving did not try to save the deleted document
  })
})
