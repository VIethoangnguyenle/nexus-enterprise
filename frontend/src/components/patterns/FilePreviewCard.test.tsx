import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import type { DriveItem } from '../../api/drive'
import { FilePreviewCard } from './FilePreviewCard'

const mockItem: DriveItem = {
  id: '33333333-3333-4333-8333-333333333333',
  workspace_id: 'ws-1',
  drive_context: 'channel',
  item_type: 'file',
  name: 'hop-dong.pdf',
  mime_type: 'application/pdf',
  size_bytes: 2048 * 1024,
  ngac_node_id: 'node-hopdong',
  owner_id: 'user-1',
  owner_name: 'Nguyễn Văn A',
  status: 'active',
  created_at: '2026-10-10T10:00:00Z',
  updated_at: '2026-10-10T10:00:00Z',
}

const mockDownload = vi.fn()
vi.mock('../../hooks/useDownloadUrl', () => ({
  useDownloadFile: () => ({ download: mockDownload, isDownloading: false }),
  useDownloadUrl: () => ({ data: 'https://minio.local/file', isError: false }),
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
  mockDownload.mockReset()
})

describe('FilePreviewCard', () => {
  it('renders card with file name and size, clicking opens FileViewer, closing returns focus', async () => {
    const user = userEvent.setup()
    renderWithClient(<FilePreviewCard item={mockItem} />)

    const card = screen.getByRole('button', { name: 'Xem trước hop-dong.pdf' })
    expect(card).toBeInTheDocument()
    expect(screen.getByText('hop-dong.pdf')).toBeInTheDocument()
    expect(screen.getByText(/PDF · 2 MB/)).toBeInTheDocument()

    // Clicking card opens FileViewer
    await user.click(card)
    const viewer = await screen.findByRole('dialog', { name: /hop-dong\.pdf/i })
    expect(viewer).toBeInTheDocument()

    // Esc closes FileViewer and returns focus to card
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(card).toHaveFocus()

    // No UUID on screen
    const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('download button triggers download without opening FileViewer', async () => {
    const user = userEvent.setup()
    renderWithClient(<FilePreviewCard item={mockItem} />)

    const downloadBtn = screen.getByRole('button', { name: 'Tải xuống hop-dong.pdf' })
    await user.click(downloadBtn)

    expect(mockDownload).toHaveBeenCalledWith({ id: mockItem.id, name: mockItem.name })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('accepts fileId and filename props and navigates list if files passed', async () => {
    const user = userEvent.setup()
    const files = [
      { id: '1', name: 'first.pdf' },
      { id: '2', name: 'second.xlsx' },
    ]
    renderWithClient(<FilePreviewCard fileId="1" filename="first.pdf" files={files} index={0} />)

    const card = screen.getByRole('button', { name: 'Xem trước first.pdf' })
    await user.click(card)

    const viewer = await screen.findByRole('dialog', { name: /first\.pdf/i })
    expect(viewer).toBeInTheDocument()

    // Next button moves to second file
    const nextBtn = within(viewer).getAllByRole('button', { name: 'Tệp sau' })[0]!
    await user.click(nextBtn)
    expect(within(viewer).getByTitle('second.xlsx')).toBeInTheDocument()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
