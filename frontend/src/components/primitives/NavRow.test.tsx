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
