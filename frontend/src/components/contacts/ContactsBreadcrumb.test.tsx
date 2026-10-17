import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ContactsBreadcrumb } from './ContactsBreadcrumb'

afterEach(cleanup)

const seg = (id: string, name: string) => ({ id, name })
const crumbs = () => screen.getByRole('navigation', { name: 'Đường dẫn' })

describe('ContactsBreadcrumb', () => {
  it('at the top shows the organisation as the current page and a building button named after it', () => {
    render(<ContactsBreadcrumb rootLabel="Toàn NovaPay" segments={[]} onOpen={() => {}} />)
    expect(within(crumbs()).getByRole('button', { name: 'Toàn NovaPay' })).toBeInTheDocument()
    expect(within(crumbs()).getByText('Toàn NovaPay', { selector: '[aria-current="page"]' })).toBeInTheDocument()
  })

  it('makes every segment but the last a button to that level, and the last the current page', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<ContactsBreadcrumb rootLabel="Toàn NovaPay" segments={[seg('k', 'Khối Vận hành'), seg('v', 'Vận hành thanh toán')]} onOpen={onOpen} />)

    expect(within(crumbs()).getByText('Vận hành thanh toán')).toHaveAttribute('aria-current', 'page')
    expect(within(crumbs()).queryByRole('button', { name: 'Vận hành thanh toán' })).toBeNull()

    await user.click(within(crumbs()).getByRole('button', { name: 'Khối Vận hành' }))
    expect(onOpen).toHaveBeenLastCalledWith('k')
    await user.click(within(crumbs()).getByRole('button', { name: 'Toàn NovaPay' }))
    expect(onOpen).toHaveBeenLastCalledWith(undefined)
  })

  it('folds the middle of a long path into …, keeps the last segments whole, and unfolds on press', async () => {
    const user = userEvent.setup()
    const path = [seg('a', 'Khối A'), seg('b', 'Phòng B'), seg('c', 'Nhóm C'), seg('d', 'Tổ D'), seg('e', 'Đội E')]
    render(<ContactsBreadcrumb rootLabel="Toàn NovaPay" segments={path} onOpen={() => {}} />)

    expect(within(crumbs()).queryByText('Phòng B')).toBeNull()
    expect(within(crumbs()).getByRole('button', { name: 'Khối A' })).toBeInTheDocument()
    expect(within(crumbs()).getByRole('button', { name: 'Tổ D' })).toBeInTheDocument()
    expect(within(crumbs()).getByText('Đội E')).toHaveAttribute('aria-current', 'page')

    await user.click(within(crumbs()).getByRole('button', { name: 'Hiện các cấp ở giữa' }))
    expect(within(crumbs()).getByRole('button', { name: 'Phòng B' })).toBeInTheDocument()
    expect(within(crumbs()).getByRole('button', { name: 'Nhóm C' })).toBeInTheDocument()
  })

  it('folds again when the path changes, so a new place never opens already unfolded', async () => {
    const user = userEvent.setup()
    const path = [seg('a', 'Khối A'), seg('b', 'Phòng B'), seg('c', 'Nhóm C'), seg('d', 'Tổ D'), seg('e', 'Đội E')]
    const { rerender } = render(<ContactsBreadcrumb rootLabel="Toàn NovaPay" segments={path} onOpen={() => {}} />)
    await user.click(within(crumbs()).getByRole('button', { name: 'Hiện các cấp ở giữa' }))
    expect(within(crumbs()).getByRole('button', { name: 'Phòng B' })).toBeInTheDocument()
    rerender(<ContactsBreadcrumb rootLabel="Toàn NovaPay" segments={[...path, seg('f', 'Nhóm F')]} onOpen={() => {}} />)
    expect(within(crumbs()).queryByRole('button', { name: 'Phòng B' })).toBeNull()
    expect(within(crumbs()).getByRole('button', { name: 'Hiện các cấp ở giữa' })).toBeInTheDocument()
  })

  it('uses its own name and trail for a role or a group, where the first step is the category', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<ContactsBreadcrumb label="Đường dẫn" leading={{ label: 'Vai trò', onPress: onOpen }} segments={[]} current="Duyệt chi" onOpen={() => {}} />)
    await user.click(within(crumbs()).getByRole('button', { name: 'Vai trò' }))
    expect(onOpen).toHaveBeenCalled()
    expect(within(crumbs()).getByText('Duyệt chi')).toHaveAttribute('aria-current', 'page')
    expect(within(crumbs()).queryByRole('button', { name: /Toàn/ })).toBeNull()
  })
})
