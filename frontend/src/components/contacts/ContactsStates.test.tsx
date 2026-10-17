import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { InlineAlert, RowSkeletons, StateCard } from './ContactsStates'

afterEach(cleanup)

describe('RowSkeletons', () => {
  it('is busy, says so to a reader, and has rows shaped like the real ones (a square for a department, a circle for a person)', () => {
    const { container } = render(<RowSkeletons shapes={['dept', 'person', 'person']} />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByText('Đang tải…')).toBeInTheDocument()
    const marks = container.querySelectorAll('[data-skeleton-mark]')
    expect([...marks].map((m) => m.getAttribute('data-skeleton-mark'))).toEqual(['dept', 'person', 'person'])
    // No spinner.
    expect(container.querySelector('svg')).toBeNull()
  })
})

describe('StateCard', () => {
  it('gives a title, a sentence and one action', async () => {
    const onPress = vi.fn()
    render(<StateCard title="Hỗ trợ đối tác chưa có ai" text="Khi quản trị viên xếp người vào phòng này, họ sẽ hiện ở đây." action={{ label: 'Về Vận hành thanh toán', onPress }} />)
    expect(screen.getByText('Hỗ trợ đối tác chưa có ai')).toBeInTheDocument()
    expect(screen.getByText(/Khi quản trị viên xếp người/)).toBeInTheDocument()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Về Vận hành thanh toán' }))
    expect(onPress).toHaveBeenCalled()
  })

  it('announces an error', () => {
    render(<StateCard tone="error" title="Không tải được danh bạ" text="Kiểm tra mạng rồi thử lại." />)
    expect(screen.getByRole('alert')).toHaveTextContent('Không tải được danh bạ')
  })
})

describe('InlineAlert', () => {
  it('says what failed in one line with Thử lại beside it', async () => {
    const onRetry = vi.fn()
    render(<InlineAlert text="Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." onRetry={onRetry} />)
    expect(screen.getByRole('alert')).toHaveTextContent('Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm.')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(onRetry).toHaveBeenCalled()
  })
})
