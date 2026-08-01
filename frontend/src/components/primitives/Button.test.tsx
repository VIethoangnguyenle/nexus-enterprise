import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Button } from './Button'

describe('Button — hình học Stitch', () => {
  it('size md dùng px-4 py-2, bo góc 8px, chữ 14/600', () => {
    render(<Button>Lưu</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('px-4')
    expect(cls).toContain('py-2')
    expect(cls).toContain('rounded-md')
    expect(cls).toContain('text-body-strong')
  })

  it('size sm là bản thu gọn của dự án, không phải của Stitch', () => {
    render(<Button size="sm">Lưu</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('px-3')
    expect(cls).toContain('py-1.5')
    expect(cls).toContain('text-small-ui')
  })

  it('size cta là nút tràn chiều rộng của Stitch: py-3, bo góc 12px', () => {
    render(<Button size="cta">Tiếp tục</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('w-full')
    expect(cls).toContain('py-3')
    expect(cls).toContain('rounded-lg')
  })
})

describe('Button — bộ variant sau khi gộp', () => {
  it('primary dùng nền primary đặc', () => {
    render(<Button variant="primary">Lưu</Button>)
    expect(screen.getByRole('button').className).toContain('bg-primary')
  })

  it('secondary là dạng có viền — Stitch gọi nó là Secondary/Outlined', () => {
    render(<Button variant="secondary">Huỷ</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('border')
    expect(cls).toContain('border-outline-variant')
  })

  it('secondary KHÔNG kèm border-none — nếu kèm thì viền biến mất mà test vẫn xanh', () => {
    render(<Button variant="secondary">Huỷ</Button>)
    // `border-none` từng nằm ở chuỗi base. Nó hoà độ đặc hiệu với `border` và
    // đứng sau trong stylesheet, nên thắng — `secondary` render không viền ở
    // mọi chỗ dùng, trong khi phép kiểm `toContain('border')` phía trên vẫn
    // xanh vì class có mặt. Đây là phép kiểm duy nhất ở tầng chuỗi bắt được nó.
    expect(screen.getByRole('button').className).not.toContain('border-none')
  })

  it('các variant không viền vẫn khai border-none tường minh', () => {
    for (const v of ['primary', 'ghost', 'danger', 'success'] as const) {
      const { unmount } = render(<Button variant={v}>x</Button>)
      expect(screen.getByRole('button').className).toContain('border-none')
      unmount()
    }
  })

  it('danger là nền đặc, không phải nền nhạt', () => {
    render(<Button variant="danger">Xoá</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-error')
    expect(cls).toContain('text-on-error')
  })

  it('success là nền đặc — đúng cái 2 màn hình đang tự chế', () => {
    render(<Button variant="success">Duyệt</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-success')
    expect(cls).toContain('text-on-success')
  })

  it('ghost trong suốt', () => {
    render(<Button variant="ghost">Bỏ qua</Button>)
    expect(screen.getByRole('button').className).toContain('bg-transparent')
  })
})

describe('Button — hành vi', () => {
  it('loading khoá nút và hiện spinner', () => {
    render(<Button loading>Lưu</Button>)
    const btn = screen.getByRole('button')
    expect(btn).toBeDisabled()
    expect(btn.querySelector('svg')).not.toBeNull()
  })
})

describe('Button — dạng liên kết', () => {
  // Bảy chỗ trong codebase tự chế nút trông như liên kết vì `sm` nhỏ nhất vẫn
  // mang px-3 py-1.5 rounded-md. `size="link"` bỏ hẳn padding và bo góc.

  it('size link bỏ padding và bo góc', () => {
    render(<Button size="link">Thử lại</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('p-0')
    expect(cls).toContain('rounded-none')
  })

  it('size link KHÔNG kèm padding của size khác', () => {
    render(<Button size="link">Thử lại</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).not.toContain('px-3')
    expect(cls).not.toContain('px-4')
    expect(cls).not.toContain('py-2')
  })

  it('variant link tô chữ primary và gạch chân khi hover', () => {
    render(<Button variant="link" size="link">Thử lại</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('text-primary')
    expect(cls).toContain('hover:underline')
    expect(cls).toContain('bg-transparent')
  })

  it('ghost + size link cho liên kết chữ trung tính', () => {
    render(<Button variant="ghost" size="link">Đăng xuất</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('text-on-surface-variant')
    expect(cls).toContain('p-0')
  })
})
