import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { D, WS_ID, calls, driveFixtureApi, resetFixtures } from '../../test/drive-fixtures'
import { validateDriveSearch } from '../../lib/drive-search'
import { apiFetch } from '../../api/client'
import type { DriveSettings } from '../../api/settings'
import { queryClient } from '../../lib/query-client'
import { settingsQueryKey } from '../../hooks/useServerSettings'
import { useFileViewStore } from '../../stores/file-view.store'
import { useDriveStore } from '../../stores/drive.store'
import { clearCreateActions } from '../../hooks/useModuleCreateAction'
import { resetUploadQueueForTests } from '../../stores/upload-queue.store'
import { DriveScreen } from './DriveScreen'

vi.mock('../../api/client', async (orig) => {
  const { driveFixtureApi } = await import('../../test/drive-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(driveFixtureApi) }
})
const api = vi.mocked(apiFetch)

async function renderDrive(url = '/drive') {
  const root = createRootRoute()
  const drive = createRoute({
    getParentRoute: () => root, path: '/drive', validateSearch: validateDriveSearch, component: DriveScreen,
  })
  const settings = createRoute({ getParentRoute: () => root, path: '/settings/$section', component: () => null })
  const router = createRouter({
    routeTree: root.addChildren([drive, settings]), history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

/** A wide screen with a mouse; jsdom has no layout, so the media queries are answered by hand. */
function desktop() {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
}

/** What the browser says about the network (`navigator.connection`). */
function connection(c: { type?: string; saveData?: boolean } | undefined) {
  Object.defineProperty(navigator, 'connection', {
    configurable: true,
    value: c && Object.assign(new EventTarget(), c),
  })
}

const seed = (s: DriveSettings) => queryClient.setQueryData(settingsQueryKey('drive', WS_ID), s)

beforeEach(() => {
  desktop()
  connection(undefined)
  useFileViewStore.setState({ modes: { drive: 'list', 'space-files': 'grid' } })
  useDriveStore.setState({ selectedItemId: null })
  // Pictures never arrive in these tests; the screen is judged around them.
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  calls.length = 0
  api.mockImplementation(driveFixtureApi)
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  connection(undefined)
  resetClient()
  clearCreateActions()
  resetUploadQueueForTests()
})

const rowNames = () =>
  within(screen.getByRole('table')).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)

/** The open folder (Đối soát: the folder 2026 and three files), once its rows are in. */
async function openFolder() {
  await renderDrive(`/drive?folder=${D.doisoat}`)
  await within(await screen.findByRole('table')).findByText('doi-soat-08-10.xlsx')
}

describe('DriveScreen: the desktop gear to Cài đặt Tài liệu', () => {
  it('on a wide screen the header ends with the gear, which opens Cài đặt Tài liệu in the same workspace', async () => {
    const user = userEvent.setup()
    const router = await renderDrive(`/drive?ws=${WS_ID}&folder=${D.doisoat}`)
    const gear = await screen.findByRole('button', { name: 'Cài đặt Tài liệu' })
    expect(gear).toHaveAttribute('title', 'Cài đặt Tài liệu')
    await user.click(gear)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/drive'))
    // Only the workspace follows; the open folder stays behind.
    expect(router.state.location.search).toEqual({ ws: WS_ID })
  })

  it('a phone has no gear: its ⋯ already leads there', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    await renderDrive()
    await screen.findByRole('heading', { name: 'Tài liệu', level: 1 })
    expect(screen.queryByRole('button', { name: 'Cài đặt Tài liệu' })).toBeNull()
  })
})

describe('DriveScreen: Cài đặt Tài liệu', () => {
  it('at the defaults the list is as before: folders first, last change first', async () => {
    seed({ view: 'auto', sort: 'modified', folders_first: true, cellular_previews: true })
    await openFolder()
    expect(rowNames()).toEqual(['2026', 'doi-soat-08-10.xlsx', 'sao-ke-ngan-hang-10-2026.pdf', 'bien-ban-chenh-lech.docx'])
  })

  it('Sắp xếp: Tên orders the files by name', async () => {
    seed({ sort: 'name' })
    await openFolder()
    await waitFor(() => expect(rowNames()).toEqual(['2026', 'bien-ban-chenh-lech.docx', 'doi-soat-08-10.xlsx', 'sao-ke-ngan-hang-10-2026.pdf']))
  })

  it('Thư mục trước tệp off: folders take their place in the one order', async () => {
    seed({ sort: 'size', folders_first: true })
    await openFolder()
    await waitFor(() => expect(rowNames()[0]).toBe('2026'))
    cleanup()
    resetClient()

    seed({ sort: 'size', folders_first: false })
    await openFolder()
    // A folder has no size of its own, so by size it comes last.
    await waitFor(() => expect(rowNames()[rowNames().length - 1]).toBe('2026'))
  })

  it('Xem dạng: Lưới and Danh sách open on that layout whatever this device last chose; Tự động keeps it', async () => {
    useFileViewStore.setState({ modes: { drive: 'list', 'space-files': 'grid' } })
    seed({ view: 'grid' })
    await renderDrive()
    expect(await screen.findByRole('group', { name: 'Tệp' })).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
    cleanup()
    resetClient()

    useFileViewStore.setState({ modes: { drive: 'grid', 'space-files': 'grid' } })
    seed({ view: 'list' })
    await renderDrive()
    expect(await screen.findByRole('table')).toBeInTheDocument()
    cleanup()
    resetClient()

    useFileViewStore.setState({ modes: { drive: 'grid', 'space-files': 'grid' } })
    seed({ view: 'auto' })
    await renderDrive()
    expect(await screen.findByRole('group', { name: 'Tệp' })).toBeInTheDocument()
  })

  it('Tải ảnh xem trước off: on mobile data no picture is asked for and the file-kind icon shows', async () => {
    connection({ type: 'cellular' })
    seed({ cellular_previews: false })
    await openFolder()
    const table = screen.getByRole('table')
    await waitFor(() => expect(within(table).getAllByTestId('file-thumb-fallback').length).toBeGreaterThan(0))
    expect(within(table).queryAllByTestId('file-thumb-skeleton')).toHaveLength(0)
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u).includes('thumbnail'))).toBe(false)
  })

  it('Tải ảnh xem trước off: with data saving on it is the same', async () => {
    connection({ type: 'wifi', saveData: true })
    seed({ cellular_previews: false })
    await openFolder()
    await waitFor(() => expect(within(screen.getByRole('table')).getAllByTestId('file-thumb-fallback').length).toBeGreaterThan(0))
  })

  it('Tải ảnh xem trước off: on Wi-Fi pictures still load', async () => {
    connection({ type: 'wifi' })
    seed({ cellular_previews: false })
    await openFolder()
    expect(within(screen.getByRole('table')).getAllByTestId('file-thumb-skeleton').length).toBeGreaterThan(0)
    expect(within(screen.getByRole('table')).queryAllByTestId('file-thumb-fallback')).toHaveLength(0)
  })

  it('Tải ảnh xem trước on: mobile data loads pictures as before', async () => {
    connection({ type: 'cellular' })
    seed({ cellular_previews: true })
    await openFolder()
    expect(within(screen.getByRole('table')).getAllByTestId('file-thumb-skeleton').length).toBeGreaterThan(0)
  })
})
