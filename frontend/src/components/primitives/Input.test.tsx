import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Input } from './Input'

describe('Input — dạng mặc định', () => {
  it('là ô chữ nhật trên nền nổi', () => {
    render(<Input aria-label="Tên" />)
    const cls = screen.getByRole('textbox').className
    expect(cls).toContain('rounded-md')
    expect(cls).toContain('bg-surface-container-lowest')
  })
})

describe('Input — dạng tìm kiếm theo Stitch §Inputs', () => {
  it('là pill trên nền surface-container-low', () => {
    render(<Input aria-label="Tìm" variant="search" />)
    const cls = screen.getByRole('textbox').className
    expect(cls).toContain('rounded-full')
    expect(cls).toContain('bg-surface-container-low')
  })

  it('không mang bo góc chữ nhật của dạng mặc định', () => {
    render(<Input aria-label="Tìm" variant="search" />)
    expect(screen.getByRole('textbox').className).not.toContain('rounded-md')
  })
})

describe('Input — trạng thái lỗi', () => {
  it('hiện thông báo lỗi và tô viền error', () => {
    render(<Input aria-label="Email" error="Email không hợp lệ" />)
    expect(screen.getByText('Email không hợp lệ')).toBeInTheDocument()
    expect(screen.getByRole('textbox').className).toContain('border-danger')
  })

  it('helpText bị lỗi che khi cả hai cùng có', () => {
    render(<Input aria-label="Email" error="Sai định dạng" helpText="Dùng email công ty" />)
    expect(screen.getByText('Sai định dạng')).toBeInTheDocument()
    expect(screen.queryByText('Dùng email công ty')).toBeNull()
  })
})
