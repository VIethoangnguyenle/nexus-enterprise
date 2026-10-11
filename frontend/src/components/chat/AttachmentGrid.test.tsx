import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderWithClient } from '../../test/render'
import { AttachmentGrid } from './AttachmentGrid'
import { UUID_RE } from '../../test/chat-fixtures'
import { driveApi } from '../../api/drive'
import { useToastStore } from '../primitives'
import type { MessageAttachment } from '../../api/messaging'

vi.mock('../../api/drive', () => ({
  driveApi: {
    getDownloadUrl: vi.fn(),
  },
}))

describe('AttachmentGrid', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useToastStore.getState().clear()
    vi.mocked(driveApi.getDownloadUrl).mockResolvedValue({
      download_url: 'https://storage.local/file',
    })
  })

  const createImage = (id: string, name: string, width = 800, height = 600): MessageAttachment => ({
    drive_file_id: id,
    file_name: name,
    mime_type: 'image/jpeg',
    size_bytes: 1.2 * 1024 * 1024,
    width,
    height,
  })

  it('renders 1, 2, 4, 6 images with layout classes and +2 tile, passes opener element', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()

    const images6 = Array.from({ length: 6 }, (_, i) =>
      createImage(`11111111-aaaa-4bbb-8ccc-00000000000${i}`, `photo-${i + 1}.jpg`),
    )

    const { container, rerender } = renderWithClient(
      <AttachmentGrid attachments={[images6[0]!]} onOpen={onOpen} />,
    )
    const firstTile = screen.getByRole('button', { name: 'Mở ảnh photo-1.jpg' })
    expect(firstTile).toBeInTheDocument()
    expect(container.textContent).not.toMatch(UUID_RE)

    // Single image container has clamped aspect ratio
    const singleWrapper = firstTile.parentElement
    expect(singleWrapper).toHaveStyle({ aspectRatio: '1.3333333333333333' })

    // 2 images
    rerender(<AttachmentGrid attachments={images6.slice(0, 2)} onOpen={onOpen} />)
    expect(screen.getAllByRole('button', { name: /Mở ảnh/ })).toHaveLength(2)

    // 4 images
    rerender(<AttachmentGrid attachments={images6.slice(0, 4)} onOpen={onOpen} />)
    expect(screen.getAllByRole('button', { name: /Mở ảnh/ })).toHaveLength(4)
    expect(screen.queryByText(/\+/)).not.toBeInTheDocument()

    // 6 images -> shows 4 tiles, 4th has "+2"
    rerender(<AttachmentGrid attachments={images6} onOpen={onOpen} />)
    expect(screen.getByText('+2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Mở ảnh photo-4.jpg, còn 2 ảnh nữa/ })).toBeInTheDocument()
    expect(container.textContent).not.toMatch(UUID_RE)

    // Clicking tile calls onOpen with (index, every file in display order, openerElement)
    await user.click(screen.getByRole('button', { name: 'Mở ảnh photo-1.jpg' }))
    expect(onOpen).toHaveBeenCalledWith(0, images6, expect.any(HTMLElement))
  })

  it('keeps measured aspect ratio for single portrait image', () => {
    const portrait = createImage('11111111-aaaa-4bbb-8ccc-000000000001', 'receipt.jpg', 600, 1200)
    renderWithClient(<AttachmentGrid attachments={[portrait]} />)
    const tile = screen.getByRole('button', { name: 'Mở ảnh receipt.jpg' })
    expect(tile.parentElement).toHaveStyle({ aspectRatio: '0.5' })
  })

  it('renders local thumbnail url immediately without fetch', () => {
    const localImg: MessageAttachment = {
      ...createImage('local-1', 'preview.jpg'),
      localUrl: 'blob:http://localhost/local-blob-url',
    }
    renderWithClient(<AttachmentGrid attachments={[localImg]} />)
    const img = screen.getByRole('img', { name: 'preview.jpg' })
    expect(img).toHaveAttribute('src', 'blob:http://localhost/local-blob-url')
    expect(driveApi.getDownloadUrl).not.toHaveBeenCalled()
  })

  it('renders a PDF card and handles download failure', async () => {
    const user = userEvent.setup()
    vi.mocked(driveApi.getDownloadUrl).mockRejectedValue(new Error('fail'))

    const pdf: MessageAttachment = {
      drive_file_id: '22222222-aaaa-4bbb-8ccc-000000000001',
      file_name: 'bien-ban.pdf',
      mime_type: 'application/pdf',
      size_bytes: 1.2 * 1024 * 1024,
    }

    const { container } = renderWithClient(<AttachmentGrid attachments={[pdf]} />)

    expect(screen.getByText('bien-ban.pdf')).toBeInTheDocument()
    expect(screen.getByText(/PDF · 1,2 MB/)).toBeInTheDocument()
    expect(container.textContent).not.toMatch(UUID_RE)

    const dlBtn = screen.getByRole('button', { name: /Tải xuống bien-ban\.pdf/ })
    await user.click(dlBtn)

    await waitFor(() => {
      const toasts = useToastStore.getState().toasts
      expect(toasts.length).toBeGreaterThan(0)
      expect(toasts[0]?.tone).toBe('error')
    })
  })

  it('lists pictures first and files after them as one sequence, and opens a file by its position in it', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    const pdf: MessageAttachment = { drive_file_id: 'p-1', file_name: 'bien-ban.pdf', mime_type: 'application/pdf', size_bytes: 2048 }
    const xls: MessageAttachment = { drive_file_id: 'x-1', file_name: 'phi.xlsx', mime_type: '', size_bytes: 4096 }
    const a = createImage('i-1', 'a.jpg')
    const b = createImage('i-2', 'b.jpg')
    // Sent in this order; shown as the pictures, then the files.
    renderWithClient(<AttachmentGrid attachments={[pdf, a, xls, b]} onOpen={onOpen} />)

    await user.click(screen.getByRole('button', { name: 'Mở ảnh b.jpg' }))
    expect(onOpen).toHaveBeenLastCalledWith(1, [a, b, pdf, xls], expect.any(HTMLElement))

    await user.click(screen.getByRole('button', { name: 'Mở tệp phi.xlsx' }))
    expect(onOpen).toHaveBeenLastCalledWith(3, [a, b, pdf, xls], expect.any(HTMLElement))
  })

  it('downloading a file does not open it', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    const pdf: MessageAttachment = { drive_file_id: 'p-1', file_name: 'bien-ban.pdf', mime_type: 'application/pdf', size_bytes: 2048 }
    renderWithClient(<AttachmentGrid attachments={[pdf]} onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: /Tải xuống bien-ban\.pdf/ }))
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('shows no id on a file card', () => {
    const pdf: MessageAttachment = { drive_file_id: '22222222-aaaa-4bbb-8ccc-000000000009', file_name: 'a.pdf', mime_type: 'application/pdf', size_bytes: 1 }
    const { container } = renderWithClient(<AttachmentGrid attachments={[pdf]} onOpen={vi.fn()} />)
    expect(container.textContent).not.toMatch(UUID_RE)
    expect(container.innerHTML).not.toMatch(UUID_RE)
  })
})
