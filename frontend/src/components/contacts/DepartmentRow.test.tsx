import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DeptNode } from './contacts-model'
import { DepartmentRow } from './DepartmentRow'

afterEach(cleanup)

const node = (over: Partial<DeptNode> = {}): DeptNode => ({
  id: 'd1', name: 'Phòng AI', parentId: 'k', hidden: false, children: [], people: [], peopleCount: 7, subDepartmentCount: 0, ...over,
})

describe('DepartmentRow', () => {
  it('shows the name and how many people are in it and beneath it', () => {
    render(<DepartmentRow node={node()} onOpen={() => {}} />)
    expect(screen.getByRole('button', { name: /Phòng AI/ })).toBeInTheDocument()
    expect(screen.getByText('7 người')).toBeInTheDocument()
  })

  it('says how many departments lie below only when there are some', () => {
    const { rerender } = render(<DepartmentRow node={node()} onOpen={() => {}} />)
    expect(screen.queryByText(/phòng, nhóm bên dưới/)).toBeNull()
    rerender(<DepartmentRow node={node({ subDepartmentCount: 3 })} onOpen={() => {}} />)
    expect(screen.getByText('3 phòng, nhóm bên dưới')).toBeInTheDocument()
  })

  it('marks a department an administrator hid from the others', () => {
    render(<DepartmentRow node={node({ hidden: true })} onOpen={() => {}} />)
    expect(screen.getByText('Ẩn khỏi danh bạ')).toBeInTheDocument()
  })

  it('opens the department it names', async () => {
    const onOpen = vi.fn()
    render(<DepartmentRow node={node()} onOpen={onOpen} />)
    await userEvent.setup().click(screen.getByRole('button'))
    expect(onOpen).toHaveBeenCalledWith('d1')
  })
})
