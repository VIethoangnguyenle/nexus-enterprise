import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderWithClient } from '../../test/render'
import { ImageViewer } from './ImageViewer'
import { UUID_RE } from '../../test/chat-fixtures'
import { driveApi } from '../../api/drive'
import { useToastStore } from '../primitives'
import type { MessageAttachment } from '../../api/messaging'

vi.mock('../../api/drive', () => ({
  driveApi: {
    getDownloadUrl: vi.fn(),
  },
}))

describe('ImageViewer', () => {
  const images: MessageAttachment[] = [
    { drive_file_id: '11111111-aaaa-4bbb-8ccc-000000000001', file_name: 'photo-1.jpg', mime_type: 'image/jpeg', size_bytes: 1.2 * 1024 * 1024 },
    { drive_file_id: '11111111-aaaa-4bbb-8ccc-000000000002', file_name: 'photo-2.jpg', mime_type: 'image/jpeg', size_bytes: 1.5 * 1024 * 1024 },
    { drive_file_id: '11111111-aaaa-4bbb-8ccc-000000000003', file_name: 'photo-3.jpg', mime_type: 'image/jpeg', size_bytes: 2.0 * 1024 * 1024 },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    useToastStore.getState().clear()
    vi.mocked(driveApi.getDownloadUrl).mockImplementation(async (id) => ({
      download_url: `https://storage.local/img/${id}`,
    }))
  })

  it('navigates with buttons and arrow keys, closes on Esc and returns focus on unmount', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()

    const trigger = document.createElement('button')
    trigger.textContent = 'Open thumbnail'
    document.body.appendChild(trigger)
    trigger.focus()
    const returnFocusRef = { current: trigger }

    const { unmount } = renderWithClient(
      <ImageViewer
        open={true}
        images={images}
        initialIndex={0}
        senderName="Lê Quang Vinh"
        sentAt="2026-10-09T09:31:00Z"
        onClose={onClose}
        returnFocusRef={returnFocusRef}
      />,
    )

    expect(screen.getByText('1 / 3')).toBeInTheDocument()
    expect(screen.getByText('photo-1.jpg')).toBeInTheDocument()
    expect(screen.getByText(/Lê Quang Vinh/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(UUID_RE)

    // Prev button is hidden at index 0
    expect(screen.queryByRole('button', { name: 'Ảnh trước' })).not.toBeInTheDocument()

    // Next button moves to 2
    const nextBtn = screen.getByRole('button', { name: 'Ảnh sau' })
    await user.click(nextBtn)
    expect(screen.getByText('2 / 3')).toBeInTheDocument()
    expect(screen.getByText('photo-2.jpg')).toBeInTheDocument()

    // Arrow keys navigate
    await user.keyboard('{ArrowRight}')
    expect(screen.getByText('3 / 3')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Ảnh sau' })).not.toBeInTheDocument()

    await user.keyboard('{ArrowLeft}')
    expect(screen.getByText('2 / 3')).toBeInTheDocument()

    // Esc closes
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()

    // Unmounting the viewer (as MessageBlock does on close) returns focus to trigger
    unmount()
    await waitFor(() => {
      expect(document.activeElement).toBe(trigger)
    })
    trigger.remove()
  })

  it('traps Tab focus inside the dialog', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()

    renderWithClient(
      <ImageViewer
        open={true}
        images={images}
        initialIndex={0}
        onClose={onClose}
      />,
    )

    const dialog = screen.getByRole('dialog', { name: 'Trình xem ảnh' })
    expect(dialog).toBeInTheDocument()

    const downloadBtn = screen.getByRole('button', { name: 'Tải xuống' })
    const closeBtn = screen.getByRole('button', { name: 'Đóng trình xem' })
    const nextBtn = screen.getByRole('button', { name: 'Ảnh sau' })

    // Tab moves between controls within dialog
    closeBtn.focus()
    await user.tab()
    expect(dialog.contains(document.activeElement)).toBe(true)

    // Tab backwards from download button wraps to last element (nextBtn)
    downloadBtn.focus()
    await user.tab({ shift: true })
    expect(document.activeElement).toBe(nextBtn)
  })

  it('shows toast on download failure', async () => {
    const user = userEvent.setup()
    vi.mocked(driveApi.getDownloadUrl).mockRejectedValue(new Error('Network error'))

    renderWithClient(
      <ImageViewer
        open={true}
        images={images}
        initialIndex={0}
        onClose={vi.fn()}
      />,
    )

    const downloadBtn = screen.getByRole('button', { name: 'Tải xuống' })
    await user.click(downloadBtn)

    await waitFor(() => {
      const toasts = useToastStore.getState().toasts
      expect(toasts.length).toBeGreaterThan(0)
      expect(toasts[0]?.tone).toBe('error')
    })
  })
})
