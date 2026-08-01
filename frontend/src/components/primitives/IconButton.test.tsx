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

  it('filled là nút tròn — cả hai chỗ dùng trong repo đều rounded-full', () => {
    render(<IconButton aria-label="Gửi" variant="filled" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('rounded-full')
    // Nếu `rounded-md` còn nằm ở chuỗi base, nó hoà độ đặc hiệu với `rounded-full`
    // và thắng theo thứ tự stylesheet — nút gửi lặng lẽ mất hình tròn trong khi
    // phép kiểm `toContain('rounded-full')` phía trên vẫn xanh. Đây là phép kiểm
    // duy nhất ở tầng chuỗi bắt được điều đó.
    expect(cls).not.toContain('rounded-md')
  })

  it('ghost và outlined mặc định giữ bo góc 8px', () => {
    for (const v of ['ghost', 'outlined'] as const) {
      const { unmount } = render(<IconButton aria-label="x" variant={v} />)
      const cls = screen.getByRole('button').className
      expect(cls).toContain('rounded-md')
      expect(cls).not.toContain('rounded-full')
      unmount()
    }
  })
})

describe('IconButton — hình dạng là trục riêng', () => {
  // Thanh soạn tin có hàng nút ghost TRÒN, trong khi Stitch §Buttons quy định
  // icon button có viền là 8px. Cùng variant, khác hình — nên hình dạng phải là
  // prop riêng chứ không gộp vào variant. `ChatEditor.tsx` từng truyền
  // className="rounded-full" và thua `rounded-md` nướng sẵn, render vuông 8px.

  it('ghost shape="round" thật sự tròn, không kèm rounded-md', () => {
    render(<IconButton aria-label="Emoji" shape="round" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('rounded-full')
    expect(cls).not.toContain('rounded-md')
  })

  it('filled mặc định tròn mà không cần khai shape', () => {
    render(<IconButton aria-label="Gửi" variant="filled" />)
    expect(screen.getByRole('button').className).toContain('rounded-full')
  })

  it('filled shape="square" ép được về vuông khi cần', () => {
    render(<IconButton aria-label="Gửi" variant="filled" shape="square" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('rounded-md')
    expect(cls).not.toContain('rounded-full')
  })

  it('outlined shape="round" giữ viền nhưng đổi hình', () => {
    render(<IconButton aria-label="Lọc" variant="outlined" shape="round" />)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('rounded-full')
    expect(cls).toContain('border-outline-variant')
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
