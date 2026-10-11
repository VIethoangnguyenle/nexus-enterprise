import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pollApi } from '../../api/chat-cards'
import { ApiError } from '../../api/client'
import { PollDialog, validatePoll } from './PollDialog'
import { renderWithClient, resetClient } from '../../test/render'

vi.mock('../../api/chat-cards', () => ({
  pollApi: { get: vi.fn(), vote: vi.fn(), unvote: vi.fn(), close: vi.fn(), create: vi.fn() },
}))

beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

const NOW = new Date('2026-10-11T04:00:00Z')

describe('validatePoll', () => {
  const ok = { question: 'Họp lúc nào?', options: ['A', 'B'], endsAt: undefined }
  it('accepts a question with two to ten filled choices', () => {
    expect(validatePoll(ok, NOW)).toEqual({})
    expect(validatePoll({ ...ok, options: Array.from({ length: 10 }, (_, i) => `Lựa chọn ${i}`) }, NOW)).toEqual({})
  })
  it('needs a question', () => {
    expect(validatePoll({ ...ok, question: '  ' }, NOW).question).toBeTruthy()
  })
  it('refuses fewer than two choices', () => {
    expect(validatePoll({ ...ok, options: ['A'] }, NOW).options).toBeTruthy()
    expect(validatePoll({ ...ok, options: [] }, NOW).options).toBeTruthy()
  })
  it('refuses more than ten', () => {
    expect(validatePoll({ ...ok, options: Array.from({ length: 11 }, (_, i) => `x${i}`) }, NOW).options).toBeTruthy()
  })
  it('refuses a blank choice and names its row', () => {
    expect(validatePoll({ ...ok, options: ['A', '  ', 'C'] }, NOW).blank).toEqual([1])
  })
  it('refuses an end time that is not in the future', () => {
    expect(validatePoll({ ...ok, endsAt: new Date('2026-10-11T03:00:00Z') }, NOW).endsAt).toBeTruthy()
    expect(validatePoll({ ...ok, endsAt: new Date('2026-10-11T05:00:00Z') }, NOW).endsAt).toBeUndefined()
  })
})

describe('PollDialog', () => {
  const open = () => renderWithClient(<PollDialog open onClose={vi.fn()} channelId="ch" />)

  it('sends nothing and says why when the choices are not enough', async () => {
    const user = userEvent.setup()
    open()
    await user.type(screen.getByLabelText('Câu hỏi'), 'Họp lúc nào?')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    expect(pollApi.create).not.toHaveBeenCalled()
    expect(await screen.findAllByText(/không được để trống|ít nhất 2 lựa chọn/i)).not.toHaveLength(0)
  })

  it('marks the blank row when another choice is filled', async () => {
    const user = userEvent.setup()
    open()
    await user.type(screen.getByLabelText('Câu hỏi'), 'Họp lúc nào?')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.click(screen.getByRole('button', { name: 'Thêm lựa chọn' }))
    await user.type(screen.getByLabelText('Lựa chọn 2'), 'Chiều')
    await user.click(screen.getByRole('button', { name: 'Thêm lựa chọn' }))
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    expect(pollApi.create).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Lựa chọn 3')).toHaveAttribute('aria-invalid', 'true')
  })

  it('stops adding at ten choices', async () => {
    const user = userEvent.setup()
    open()
    for (let i = 0; i < 12; i++) {
      const add = screen.getByRole('button', { name: 'Thêm lựa chọn' })
      if (add.hasAttribute('disabled')) break
      await user.click(add)
    }
    expect(screen.getAllByLabelText(/^Lựa chọn \d+$/)).toHaveLength(10)
    expect(screen.getByRole('button', { name: 'Thêm lựa chọn' })).toBeDisabled()
  })

  it('removes a choice row', async () => {
    const user = userEvent.setup()
    open()
    await user.click(screen.getByRole('button', { name: 'Thêm lựa chọn' }))
    await user.click(screen.getByRole('button', { name: 'Bỏ lựa chọn 3' }))
    expect(screen.getAllByLabelText(/^Lựa chọn \d+$/)).toHaveLength(2)
  })

  it('refuses a past end time and sends nothing', async () => {
    const user = userEvent.setup()
    open()
    await user.type(screen.getByLabelText('Câu hỏi'), 'Họp lúc nào?')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.type(screen.getByLabelText('Lựa chọn 2'), 'Chiều')
    await user.selectOptions(screen.getByLabelText('Đóng bình chọn'), 'custom')
    await user.type(screen.getByLabelText('Ngày giờ đóng'), '2020-01-01T08:00')
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    expect(pollApi.create).not.toHaveBeenCalled()
    expect(await screen.findByText(/phải ở tương lai/i)).toBeInTheDocument()
  })

  it('sends a valid poll with its switches and closes', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(pollApi.create).mockResolvedValue({} as never)
    renderWithClient(<PollDialog open onClose={onClose} channelId="ch" />)
    await user.type(screen.getByLabelText('Câu hỏi'), ' Họp lúc nào? ')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.type(screen.getByLabelText('Lựa chọn 2'), 'Chiều')
    await user.click(screen.getByLabelText('Cho chọn nhiều đáp án'))
    await user.click(screen.getByLabelText(/Ẩn danh/))
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    await waitFor(() => expect(pollApi.create).toHaveBeenCalledWith('ch', {
      question: 'Họp lúc nào?', options: ['Sáng', 'Chiều'], is_multi: true, is_anonymous: true,
    }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('keeps the form and explains a refusal', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(pollApi.create).mockRejectedValue(new ApiError('forbidden', 403))
    renderWithClient(<PollDialog open onClose={onClose} channelId="ch" />)
    await user.type(screen.getByLabelText('Câu hỏi'), 'Họp lúc nào?')
    await user.type(screen.getByLabelText('Lựa chọn 1'), 'Sáng')
    await user.type(screen.getByLabelText('Lựa chọn 2'), 'Chiều')
    await user.click(screen.getByRole('button', { name: 'Gửi bình chọn' }))
    expect(await screen.findByText(/Bạn chưa có quyền tạo bình chọn/)).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Câu hỏi')).toHaveValue('Họp lúc nào?')
  })
})
