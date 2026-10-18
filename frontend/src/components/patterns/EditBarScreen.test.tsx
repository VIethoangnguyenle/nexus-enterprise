import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { barModules, defaultBar, type BarModuleId } from '../../lib/nav-destinations'
import { EditBarScreen } from './EditBarScreen'

afterEach(cleanup)

const show = (over: { personal?: boolean; bar?: BarModuleId[]; onSave?: (bar: BarModuleId[]) => Promise<boolean> } = {}) => {
  const personal = over.personal ?? false
  const onSave = vi.fn(over.onSave ?? (() => Promise.resolve(true)))
  const onClose = vi.fn()
  render(
    <EditBarScreen
      modules={barModules(personal)}
      bar={over.bar ?? defaultBar(personal)}
      defaults={defaultBar(personal)}
      onSave={onSave}
      onClose={onClose}
    />,
  )
  const dialog = screen.getByRole('dialog', { name: 'Sửa thanh đáy' })
  const onBar = () => within(within(dialog).getAllByRole('list')[0]!).getAllByRole('listitem').map((li) => li.textContent?.replace(/Thay$/, ''))
  return { dialog, onSave, onClose, onBar, user: userEvent.setup() }
}

describe('Sửa thanh đáy', () => {
  it('lists the four on the bar in order, and the rest under Trong Thêm', () => {
    const { dialog, onBar } = show()
    expect(within(dialog).getByText('Ở thanh đáy (4/4)')).toBeInTheDocument()
    expect(onBar()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt'])
    const rest = within(within(dialog).getAllByRole('list')[1]!).getAllByRole('listitem')
    expect(rest.map((li) => li.textContent?.replace(/Đưa lên$/, ''))).toEqual(['Tài sản', 'Danh bạ', 'Thông báo'])
  })

  it('never offers Phê duyệt or Tài sản in a personal workspace', () => {
    const { dialog, onBar } = show({ personal: true })
    expect(onBar()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Danh bạ'])
    expect(within(dialog).queryByText('Phê duyệt')).toBeNull()
    expect(within(dialog).queryByText('Tài sản')).toBeNull()
    expect(within(dialog).queryByText('Quản trị')).toBeNull()
  })

  it('moves a module up and down with the arrow buttons, and the ends cannot go further', async () => {
    const { dialog, onBar, user } = show()
    expect(within(dialog).getByRole('button', { name: 'Đưa Chat lên trước' })).toBeDisabled()
    expect(within(dialog).getByRole('button', { name: 'Đưa Phê duyệt xuống sau' })).toBeDisabled()
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Tài liệu lên trước' }))
    expect(onBar()).toEqual(['Chat', 'Tài liệu', 'Lịch', 'Phê duyệt'])
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Chat xuống sau' }))
    expect(onBar()).toEqual(['Tài liệu', 'Chat', 'Lịch', 'Phê duyệt'])
  })

  it('brings a module up by naming the one it replaces, and the bar stays at four', async () => {
    const { dialog, onBar, user } = show()
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Tài sản lên thanh đáy' }))
    expect(within(dialog).getByText('Chọn phân hệ trên thanh đáy để thay bằng Tài sản.')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Thay Lịch bằng Tài sản' }))
    expect(onBar()).toEqual(['Chat', 'Tài sản', 'Tài liệu', 'Phê duyệt'])
    expect(within(dialog).getByText('Ở thanh đáy (4/4)')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Đưa Lịch lên thanh đáy' })).toBeInTheDocument()
  })

  it('lets the person change their mind before naming the module to replace', async () => {
    const { dialog, onBar, user } = show()
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Tài sản lên thanh đáy' }))
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    expect(onBar()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt'])
    expect(within(dialog).queryByRole('button', { name: /^Thay / })).toBeNull()
  })

  it('goes back to the defaults of the workspace kind', async () => {
    const { dialog, onBar, user } = show({ bar: ['notifications', 'assets', 'chat', 'drive'] })
    await user.click(within(dialog).getByRole('button', { name: 'Về mặc định' }))
    expect(onBar()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt'])
  })

  it('saves the draft on Xong and closes', async () => {
    const { dialog, onSave, onClose, user } = show()
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await user.click(within(dialog).getByRole('button', { name: 'Xong' }))
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
    expect(onSave).toHaveBeenCalledWith(['calendar', 'chat', 'drive', 'approval'])
  })

  it('saves nothing for an unchanged bar', async () => {
    const { dialog, onSave, onClose, user } = show()
    await user.click(within(dialog).getByRole('button', { name: 'Xong' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onSave).not.toHaveBeenCalled()
  })

  it('stays open when the save is refused, so nothing the person arranged is lost', async () => {
    const { dialog, onClose, onBar, user } = show({ onSave: () => Promise.resolve(false) })
    await user.click(within(dialog).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await user.click(within(dialog).getByRole('button', { name: 'Xong' }))
    await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Xong' })).toBeEnabled())
    expect(onClose).not.toHaveBeenCalled()
    expect(onBar()).toEqual(['Lịch', 'Chat', 'Tài liệu', 'Phê duyệt'])
  })

  it('leaves without saving on Quay lại and on Esc', async () => {
    const first = show()
    await first.user.click(within(first.dialog).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await first.user.click(within(first.dialog).getByRole('button', { name: 'Quay lại' }))
    expect(first.onClose).toHaveBeenCalledTimes(1)
    expect(first.onSave).not.toHaveBeenCalled()
    cleanup()
    const second = show()
    await second.user.keyboard('{Escape}')
    expect(second.onClose).toHaveBeenCalledTimes(1)
    expect(second.onSave).not.toHaveBeenCalled()
  })
})
