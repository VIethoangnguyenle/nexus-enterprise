import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { ImagePreviewCard } from './ImagePreviewCard'

const mockDownload = vi.fn()
vi.mock('../../hooks/useDownloadUrl', () => ({
  useDownloadFile: () => ({ download: mockDownload, isDownloading: false }),
  useDownloadUrl: () => ({ data: 'https://minio.local/photo.png', isError: false }),
}))

vi.mock('../../api/client', async (orig) => {
  const actual = await orig<typeof import('../../api/client')>()
  return {
    ...actual,
    apiFetch: vi.fn((path: string) => {
      if (path.includes('/preview')) {
        return Promise.resolve({ kind: 'unavailable', reason: 'unsupported' })
      }
      return Promise.resolve({})
    }),
  }
})

afterEach(() => {
  resetClient()
})

describe('ImagePreviewCard', () => {
  it('renders thumbnail, clicking image opens FileViewer, closing returns focus', async () => {
    const user = userEvent.setup()
    renderWithClient(<ImagePreviewCard fileId="img-1" filename="photo.png" />)

    const btn = await screen.findByRole('button', { name: 'Mở ảnh photo.png' })
    expect(btn).toBeInTheDocument()

    // Click opens FileViewer
    await user.click(btn)

    const viewer = await screen.findByRole('dialog', { name: /photo\.png/i })
    expect(viewer).toBeInTheDocument()

    // Esc closes FileViewer and returns focus
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(btn).toHaveFocus()

    // No UUID on screen
    const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('supports caller passing a files list', async () => {
    const user = userEvent.setup()
    const files = [
      { id: 'img-1', name: 'photo-1.png' },
      { id: 'img-2', name: 'photo-2.png' },
    ]
    renderWithClient(<ImagePreviewCard fileId="img-1" filename="photo-1.png" files={files} index={0} />)

    const btn = await screen.findByRole('button', { name: 'Mở ảnh photo-1.png' })
    await user.click(btn)

    const viewer = await screen.findByRole('dialog', { name: /photo-1\.png/i })
    expect(viewer).toBeInTheDocument()

    const nextBtn = within(viewer).getAllByRole('button', { name: 'Tệp sau' })[0]!
    await user.click(nextBtn)
    expect(within(viewer).getByTitle('photo-2.png')).toBeInTheDocument()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
