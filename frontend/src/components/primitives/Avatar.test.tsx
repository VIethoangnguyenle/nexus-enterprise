import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { Avatar, avatarVariant } from './Avatar'

afterEach(cleanup)

const PIC = '/api/auth/avatars/0123456789abcdef0123456789abcdef'

describe('avatarVariant', () => {
  it('asks for the small picture for small avatars and the medium one above 32 px', () => {
    expect(avatarVariant(PIC, 20)).toBe(`${PIC}/64`)
    expect(avatarVariant(PIC, 32)).toBe(`${PIC}/64`)
    expect(avatarVariant(PIC, 40)).toBe(`${PIC}/192`)
    expect(avatarVariant(PIC, 64)).toBe(`${PIC}/192`)
  })

  it('leaves any other address as it is', () => {
    expect(avatarVariant('https://cdn.example.test/a.png', 24)).toBe('https://cdn.example.test/a.png')
    expect(avatarVariant(`${PIC}/1024`, 24)).toBe(`${PIC}/1024`)
  })
})

describe('Avatar', () => {
  it('loads only the size it shows', () => {
    const { container } = render(<Avatar name="Lê Thị Hoa" src={PIC} size={24} />)
    expect(container.querySelector('img')).toHaveAttribute('src', `${PIC}/64`)
    cleanup()
    const big = render(<Avatar name="Lê Thị Hoa" src={PIC} size={64} />)
    expect(big.container.querySelector('img')).toHaveAttribute('src', `${PIC}/192`)
  })

  it('shows the initials when the picture does not load, never an empty circle', () => {
    const { container } = render(<Avatar name="Lê Thị Hoa" src={PIC} size={32} />)
    fireEvent.error(container.querySelector('img')!)
    expect(container.querySelector('img')).toBeNull()
    expect(container).toHaveTextContent('TH')
  })

  it('tries again when the picture changes', () => {
    const { container, rerender } = render(<Avatar name="Lê Thị Hoa" src={PIC} size={32} />)
    fireEvent.error(container.querySelector('img')!)
    const next = '/api/auth/avatars/fedcba9876543210fedcba9876543210'
    rerender(<Avatar name="Lê Thị Hoa" src={next} size={32} />)
    expect(container.querySelector('img')).toHaveAttribute('src', `${next}/64`)
  })

  it('shows the initials when there is no picture', () => {
    const { container } = render(<Avatar name="Trần Minh Đức" size={32} />)
    expect(container.querySelector('img')).toBeNull()
    expect(container).toHaveTextContent('MĐ')
  })
})

describe('Avatar shape', () => {
  it('is a circle unless a square is asked for', () => {
    const { container } = render(<Avatar name="Lê Thị Hoa" size={64} />)
    expect(container.firstElementChild?.className).toContain('rounded-full')
  })

  it('rounds a square by a quarter of its side and lets the picture follow', () => {
    const { container } = render(<Avatar name="Đối soát" src={PIC} size={64} shape="square" />)
    const box = container.firstElementChild as HTMLElement
    expect(box.className).not.toContain('rounded-full')
    expect(box.style.borderRadius).toBe('16px')
    expect(container.querySelector('img')?.className).toContain('rounded-[inherit]')
  })
})
