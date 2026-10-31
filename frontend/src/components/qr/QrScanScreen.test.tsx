import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { QrScanScreen } from './QrScanScreen'

// The camera library, reduced to what the screen uses; `decode` plays a scan.
const camera = vi.hoisted(() => ({
  start: vi.fn<() => Promise<void>>(),
  onDecode: null as null | ((r: { data: string }) => void),
  scanImage: vi.fn<(f: File) => Promise<{ data: string }>>(),
}))

vi.mock('qr-scanner', () => {
  class FakeScanner {
    static scanImage = (f: File) => camera.scanImage(f)
    constructor(_video: HTMLVideoElement, onDecode: (r: { data: string }) => void) {
      camera.onDecode = onDecode
    }
    start() { return camera.start() }
    stop() {}
    destroy() {}
    hasFlash() { return Promise.resolve(false) }
    toggleFlash() { return Promise.resolve() }
    isFlashOn() { return false }
  }
  return { default: FakeScanner }
})

const TOKEN = 'Abc_def-0123456789abcdefghijklmnopqrstuvwxy'

async function open() {
  const root = createRootRoute({ component: Outlet })
  const scan = createRoute({ getParentRoute: () => root, path: '/scan', component: QrScanScreen })
  const qr = createRoute({
    getParentRoute: () => root, path: '/qr/$token',
    component: function Qr() { return <div data-testid="arrived">{qr.useParams().token}</div> },
  })
  const home = createRoute({ getParentRoute: () => root, path: '/', component: () => <div>home</div> })
  const router = createRouter({
    routeTree: root.addChildren([scan, qr, home]),
    history: createMemoryHistory({ initialEntries: ['/scan'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  camera.start.mockReset().mockResolvedValue(undefined)
  camera.scanImage.mockReset()
  camera.onDecode = null
})
afterEach(() => vi.restoreAllMocks())

describe('The in-app QR scanner', () => {
  it('opens the confirm screen for a Nexus sign-in code', async () => {
    const router = await open()
    await waitFor(() => expect(camera.onDecode).not.toBeNull())
    act(() => camera.onDecode!({ data: `${window.location.origin}/qr/${TOKEN}` }))
    expect(await screen.findByTestId('arrived')).toHaveTextContent(TOKEN)
    expect(router.state.location.pathname).toBe(`/qr/${TOKEN}`)
  })

  it('says a code is not ours and keeps scanning', async () => {
    const router = await open()
    await waitFor(() => expect(camera.onDecode).not.toBeNull())
    act(() => camera.onDecode!({ data: `https://evil.example/qr/${TOKEN}` }))
    expect(await screen.findByText('Đây không phải mã đăng nhập Nexus')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/scan')
  })

  it('explains a blocked camera and offers a picture instead', async () => {
    camera.start.mockRejectedValue(new Error('NotAllowedError'))
    await open()
    expect(await screen.findByRole('heading', { name: 'Chưa có quyền dùng camera' })).toBeInTheDocument()

    camera.scanImage.mockResolvedValue({ data: `${window.location.origin}/qr/${TOKEN}` })
    const input = document.querySelector<HTMLInputElement>('input[type="file"]')!
    await userEvent.upload(input, new File(['x'], 'shot.png', { type: 'image/png' }))
    expect(await screen.findByTestId('arrived')).toHaveTextContent(TOKEN)
  })

  it('starts the camera again on Thử lại', async () => {
    camera.start.mockRejectedValueOnce(new Error('NotAllowedError'))
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Thử lại' }))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Chưa có quyền dùng camera' })).not.toBeInTheDocument())
    expect(camera.start).toHaveBeenCalledTimes(2)
  })
})
