import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { IconButton } from './IconButton'

describe('IconButton — variant', () => {
  it('mặc định là ghost, nền trong suốt', () => {
    render(<IconButton aria-label="Đóng" />)
    expect(screen.getByRole('button').className).toContain('bg-transparent')
  })

  it('outlined có viền theo Stitch §Buttons', () => {
    render(<IconButton aria-label="Lọc" variant="outlined" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('border')
    expect(cls).toContain('border-outline-variant')
  })

  it('filled dùng nền primary đặc', () => {
    render(<IconButton aria-label="Gửi" variant="filled" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-primary')
    expect(cls).toContain('text-on-primary')
  })
})

describe('IconButton — tone ngữ nghĩa', () => {
  it('tone success tô chữ màu success', () => {
    render(<IconButton aria-label="Duyệt" tone="success" />)
    expect(screen.getByRole('button').className).toContain('text-success')
  })

  it('tone danger tô chữ màu error', () => {
    render(<IconButton aria-label="Từ chối" tone="danger" />)
    expect(screen.getByRole('button').className).toContain('text-error')
  })

  it('tone mặc định dùng màu outline trung tính', () => {
    render(<IconButton aria-label="Thêm" />)
    expect(screen.getByRole('button').className).toContain('text-outline')
  })
})

describe('IconButton — kích thước', () => {
  it('mặc định là md, ô vuông 8 đơn vị', () => {
    render(<IconButton aria-label="Đóng" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('w-8')
    expect(cls).toContain('h-8')
  })
})
