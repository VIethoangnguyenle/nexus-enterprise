import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { AttachmentTray } from './AttachmentTray'
import type { AttachmentUploadItem } from '../../hooks/useAttachmentUploads'

describe('AttachmentTray', () => {
  it('renders image thumbnail and document chip with correct metadata and actions', async () => {
    const user = userEvent.setup()
    const onRemove = vi.fn()
    const onRetry = vi.fn()

    const imgFile = new File(['img'], 'photo.png', { type: 'image/png' })
    const pdfFile = new File(['pdf'], 'report.pdf', { type: 'application/pdf' })

    const items: AttachmentUploadItem[] = [
      { key: '1', file: imgFile, status: 'done', fileId: 'file-1' },
      { key: '2', file: pdfFile, status: 'done', fileId: 'file-2' },
    ]

    render(<AttachmentTray items={items} onRemove={onRemove} onRetry={onRetry} />)

    expect(screen.getByRole('img', { name: 'photo.png' })).toBeInTheDocument()
    expect(screen.getByText('report.pdf')).toBeInTheDocument()
    expect(screen.getByText(/PDF ·/)).toBeInTheDocument()

    const removeBtn = screen.getByRole('button', { name: 'Bỏ report.pdf' })
    await user.click(removeBtn)
    expect(onRemove).toHaveBeenCalledWith('2')
  })

  it('renders uploading items with real progress percentage', () => {
    const docFile = new File(['data'], 'table.xlsx', { type: 'application/vnd.ms-excel' })
    const imgFile = new File(['img'], 'photo.jpg', { type: 'image/jpeg' })

    const items: AttachmentUploadItem[] = [
      { key: '1', file: docFile, status: 'uploading', progress: 64 },
      { key: '2', file: imgFile, status: 'uploading', progress: 85 },
    ]

    render(<AttachmentTray items={items} onRemove={vi.fn()} onRetry={vi.fn()} />)

    expect(screen.getByText('Đang tải 64%')).toBeInTheDocument()
    const imgGroup = screen.getByRole('group', { name: /Đang tải 85%/ })
    expect(imgGroup).toBeInTheDocument()
  })

  it('renders errored item with Vietnamese reason and retry button', async () => {
    const user = userEvent.setup()
    const onRemove = vi.fn()
    const onRetry = vi.fn()

    const docFile = new File(['doc'], 'contract.docx', { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' })

    const items: AttachmentUploadItem[] = [
      { key: '3', file: docFile, status: 'error', error: 'Mất kết nối, chưa tải lên' },
    ]

    render(<AttachmentTray items={items} onRemove={onRemove} onRetry={onRetry} />)

    expect(screen.getByText('Mất kết nối, chưa tải lên')).toBeInTheDocument()
    const retryBtn = screen.getByRole('button', { name: 'Thử lại contract.docx' })
    await user.click(retryBtn)
    expect(onRetry).toHaveBeenCalledWith('3')
  })
})
