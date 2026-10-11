import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CreateMenuItems } from './CreateMenuItems'

let isPersonal = false
vi.mock('../../hooks/useWorkspaceType', () => ({ useWorkspaceType: () => ({ type: undefined, isPersonal, isLoading: false }) }))
let phone = false
vi.mock('../../hooks/usePhone', () => ({ usePhone: () => phone }))

beforeEach(() => {
  isPersonal = false
  phone = false
})

describe('CreateMenuItems', () => {
  it('offers poll, task, new document and approval request in an organization', () => {
    render(<CreateMenuItems onSelect={vi.fn()} />)
    expect(screen.getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Bình chọn', 'Công việc', 'Tài liệu mới', 'Đề nghị phê duyệt',
    ])
  })

  it('hides the approval request in a personal workspace', () => {
    isPersonal = true
    render(<CreateMenuItems onSelect={vi.fn()} />)
    expect(screen.queryByRole('menuitem', { name: 'Đề nghị phê duyệt' })).toBeNull()
    expect(screen.getAllByRole('menuitem')).toHaveLength(3)
  })

  it('reports which kind was chosen', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<CreateMenuItems onSelect={onSelect} />)
    await user.click(screen.getByRole('menuitem', { name: 'Tài liệu mới' }))
    expect(onSelect).toHaveBeenCalledWith('doc')
    await user.click(screen.getByRole('menuitem', { name: 'Đề nghị phê duyệt' }))
    expect(onSelect).toHaveBeenCalledWith('approval')
  })

  it('makes each row a 44px target on a phone', () => {
    phone = true
    render(<CreateMenuItems onSelect={vi.fn()} />)
    for (const i of screen.getAllByRole('menuitem')) expect(i.className).toContain('h-11')
  })
})
