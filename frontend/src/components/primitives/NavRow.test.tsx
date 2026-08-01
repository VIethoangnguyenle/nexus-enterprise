import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { NavRow } from './NavRow'

describe('NavRow — trạng thái theo Stitch §Navigation Sidebar', () => {
  it('sub-item đang active dùng nền primary-fixed và chữ on-primary-fixed-variant', () => {
    render(<NavRow kind="subItem" active>Kỹ thuật</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-primary-fixed')
    expect(cls).toContain('text-on-primary-fixed-variant')
  })

  it('group header đang active dùng nền surface-container và chữ primary', () => {
    render(<NavRow kind="groupHeader" active>Phòng ban</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-surface-container')
    expect(cls).toContain('text-primary')
  })

  it('trạng thái không active dùng chữ on-surface-variant', () => {
    render(<NavRow kind="subItem">Kinh doanh</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('text-on-surface-variant')
    expect(cls).not.toContain('bg-primary-fixed')
  })

  it('sub-item thụt lề pl-9 pr-2 đúng Stitch', () => {
    render(<NavRow kind="subItem">Kỹ thuật</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('pl-9')
    expect(cls).toContain('pr-2')
  })

  it('group header không thụt lề như sub-item', () => {
    render(<NavRow kind="groupHeader">Phòng ban</NavRow>)
    expect(screen.getByRole('button').className).not.toContain('pl-9')
  })
})

describe('NavRow — kind navItem (điều hướng phẳng, nexus-chat)', () => {
  // `.stitch/DESIGN.md` §Navigation Sidebar mô tả cây group-header + mục con thụt
  // lề — đó là sidebar Contacts. Sidebar chính của app theo màn `nexus-chat`: danh
  // sách icon phẳng, không thụt lề, hình học khác hẳn. Đo trên AppSidebar cho ra
  // gap-3 / py-2.5 px-3 / rounded-lg / font-semibold, active nền
  // surface-container-highest kèm shadow-sm. Đây là kind thứ ba, không phải biến
  // thể của hai kind kia.

  it('active dùng surface-container-highest kèm shadow, không phải primary-fixed', () => {
    render(<NavRow kind="navItem" active>Drive</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-surface-container-highest')
    expect(cls).toContain('text-primary')
    expect(cls).toContain('shadow-sm')
    expect(cls).not.toContain('bg-primary-fixed')
  })

  it('không thụt lề, dùng gap-3 và bo góc 12px', () => {
    render(<NavRow kind="navItem">Drive</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('gap-3')
    expect(cls).toContain('rounded-lg')
    expect(cls).toContain('py-2.5')
    expect(cls).not.toContain('pl-9')
  })

  it('KHÔNG kèm gap-2 hay rounded-md — nếu kèm thì hình học sụp về kind khác mà test vẫn xanh', () => {
    render(<NavRow kind="navItem">Drive</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).not.toContain('gap-2')
    expect(cls).not.toContain('rounded-md')
  })

  it('inactive đổi cả nền lẫn màu chữ khi hover, khác hai kind kia', () => {
    render(<NavRow kind="navItem">Drive</NavRow>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('hover:bg-surface-container')
    expect(cls).toContain('hover:text-on-surface')
  })
})

describe('NavRow — hành vi', () => {
  it('bấm thì gọi onClick', async () => {
    const onClick = vi.fn()
    render(<NavRow kind="subItem" onClick={onClick}>Kỹ thuật</NavRow>)
    await userEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('active được phơi ra cho trình đọc màn hình', () => {
    render(<NavRow kind="subItem" active>Kỹ thuật</NavRow>)
    expect(screen.getByRole('button')).toHaveAttribute('aria-current', 'page')
  })

  it('không active thì không đặt aria-current', () => {
    render(<NavRow kind="subItem">Kỹ thuật</NavRow>)
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-current')
  })
})
