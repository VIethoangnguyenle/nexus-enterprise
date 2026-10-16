import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { FileThumb, thumbnailSizeFor } from './FileThumb'
import { useAuthStore } from '../../stores/auth.store'

const fetchMock = vi.fn()

function renderThumb(props: Partial<React.ComponentProps<typeof FileThumb>> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <FileThumb fileId="f1" name="Hợp đồng.pdf" mimeType="application/pdf" {...props} />
    </QueryClientProvider>,
  )
}

// jsdom's Blob is not accepted by Node's Response, so a 200 is a plain stand-in.
const jpeg = () => ({ status: 200, blob: async () => new Blob(['x'], { type: 'image/jpeg' }) }) as unknown as Response

describe('thumbnailSizeFor', () => {
  it('uses 320 for cards on a 1x screen and 640 on a denser one; panels are always 640', () => {
    expect(thumbnailSizeFor('card', 1)).toBe(320)
    expect(thumbnailSizeFor('card', 2)).toBe(640)
    expect(thumbnailSizeFor('panel', 1)).toBe(640)
    expect(thumbnailSizeFor('panel', 3)).toBe(640)
  })
})

describe('FileThumb', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    URL.createObjectURL = vi.fn(() => 'blob:thumb')
    URL.revokeObjectURL = vi.fn()
    useAuthStore.setState({ accessToken: 'tok' })
  })
  afterEach(() => vi.unstubAllGlobals())

  it('reserves a 16:10 frame and shows a skeleton until the picture arrives, then the image', async () => {
    let resolve!: (r: Response) => void
    fetchMock.mockReturnValue(new Promise<Response>((r) => { resolve = r }))
    renderThumb()

    const frame = screen.getByTestId('file-thumb')
    expect(frame).toHaveStyle({ aspectRatio: '16 / 10' })
    expect(screen.getByTestId('file-thumb-skeleton')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()

    resolve(jpeg())
    const img = await screen.findByRole('img', { name: 'Bản xem trước của Hợp đồng.pdf' })
    expect(img).toHaveAttribute('src', 'blob:thumb')
    expect(img).toHaveAttribute('loading', 'lazy')
    expect(screen.queryByTestId('file-thumb-skeleton')).not.toBeInTheDocument()
    expect(frame).toHaveStyle({ aspectRatio: '16 / 10' })
  })

  it('asks for the right size with the session token', async () => {
    fetchMock.mockResolvedValue(jpeg())
    renderThumb({ size: 'panel' })
    await screen.findByRole('img')
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toBe('/api/drive/files/f1/thumbnail?size=640')
    expect(init.headers).toEqual({ Authorization: 'Bearer tok' })
  })

  it.each([
    [404, { reason: 'thumbnail_unavailable' }],
    [503, {}],
    [403, {}],
  ])('shows the file-kind icon tile, no error, when the server answers %i', async (status, body) => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify(body), { status }))
    renderThumb()
    const tile = await screen.findByTestId('file-thumb-fallback')
    expect(tile).toHaveClass('bg-sunk')
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByTestId('file-thumb-skeleton')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('file-thumb')).toHaveStyle({ aspectRatio: '16 / 10' })
  })

  it('shows the icon tile when the network fails', async () => {
    fetchMock.mockRejectedValue(new TypeError('offline'))
    renderThumb()
    await waitFor(() => expect(screen.getByTestId('file-thumb-fallback')).toBeInTheDocument())
  })

  it('keeps another aspect when asked, so the panel can use 4:3', () => {
    fetchMock.mockReturnValue(new Promise(() => {}))
    renderThumb({ aspect: '4 / 3' })
    expect(screen.getByTestId('file-thumb')).toHaveStyle({ aspectRatio: '4 / 3' })
  })

  it('marks a video with a play badge once its frame is shown', async () => {
    fetchMock.mockResolvedValue(jpeg())
    renderThumb({ name: 'Họp quý.mp4', mimeType: 'video/mp4' })
    await screen.findByRole('img')
    expect(screen.getByTestId('file-thumb-play')).toBeInTheDocument()
  })
})
