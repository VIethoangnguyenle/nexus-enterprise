import { useState } from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { updatePreferences } from '../../lib/preferences'
import { OtpInput } from './OtpInput'

type Props = Partial<Parameters<typeof OtpInput>[0]>

/** The real input is controlled by its parent, as on the sign-in screen. */
function Harness({ onComplete, onChange, initial = '', ...rest }: Props & { initial?: string }) {
  const [value, setValue] = useState(initial)
  return (
    <OtpInput
      label="Mã xác minh"
      value={value}
      onChange={(v) => {
        setValue(v)
        onChange?.(v)
      }}
      onComplete={onComplete}
      {...rest}
    />
  )
}

const input = () => screen.getByLabelText('Mã xác minh') as HTMLInputElement
const focusInput = () => act(() => input().focus())
const boxes = () => Array.from(document.querySelectorAll<HTMLElement>('[data-otp-box]'))

describe('OtpInput', () => {
  it('is one real numeric input that can take an SMS autofill, drawn as six boxes', () => {
    render(<Harness />)
    expect(input()).toHaveAttribute('inputmode', 'numeric')
    expect(input()).toHaveAttribute('autocomplete', 'one-time-code')
    expect(boxes()).toHaveLength(6)
    expect(boxes().every((b) => b.getAttribute('aria-hidden') === 'true')).toBe(true)
  })

  it('shows each digit in its own box, in order', () => {
    render(<Harness initial="481" />)
    expect(boxes().map((b) => b.textContent)).toEqual(['4', '8', '1', '', '', ''])
  })

  it('accepts digits only', async () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    await userEvent.type(input(), 'a4b8-1')
    expect(input().value).toBe('481')
  })

  it('never holds more than six digits', async () => {
    render(<Harness />)
    await userEvent.type(input(), '12345678')
    expect(input().value).toBe('123456')
  })

  it('submits once, when the sixth digit arrives', async () => {
    const onComplete = vi.fn()
    render(<Harness onComplete={onComplete} />)
    await userEvent.type(input(), '48120')
    expect(onComplete).not.toHaveBeenCalled()
    await userEvent.type(input(), '9')
    expect(onComplete).toHaveBeenCalledTimes(1)
    expect(onComplete).toHaveBeenCalledWith('481209')
  })

  describe('paste', () => {
    it('fills every box from a pasted code and submits it', async () => {
      const onComplete = vi.fn()
      render(<Harness onComplete={onComplete} />)
      focusInput()
      await userEvent.paste('481209')
      expect(boxes().map((b) => b.textContent)).toEqual(['4', '8', '1', '2', '0', '9'])
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onComplete).toHaveBeenCalledWith('481209')
    })

    it('finds the digits in a copied message and ignores the rest', async () => {
      const onComplete = vi.fn()
      render(<Harness onComplete={onComplete} />)
      focusInput()
      await userEvent.paste('Mã của bạn là 481 209, có hiệu lực 5 phút')
      // "481 209" then "5": the first six digits win.
      expect(input().value).toBe('481209')
      expect(onComplete).toHaveBeenCalledWith('481209')
    })

    it('keeps a partial paste and waits for the rest', async () => {
      const onComplete = vi.fn()
      render(<Harness onComplete={onComplete} />)
      focusInput()
      await userEvent.paste('4812')
      expect(input().value).toBe('4812')
      expect(onComplete).not.toHaveBeenCalled()
      await userEvent.type(input(), '09')
      expect(onComplete).toHaveBeenCalledWith('481209')
    })

    it('ignores a paste with no digits', async () => {
      const onChange = vi.fn()
      render(<Harness onChange={onChange} />)
      focusInput()
      await userEvent.paste('không có số')
      expect(onChange).not.toHaveBeenCalled()
    })
  })

  describe('keyboard', () => {
    it('Backspace removes the last digit', async () => {
      render(<Harness initial="481" />)
      focusInput()
      act(() => input().setSelectionRange(3, 3))
      await userEvent.keyboard('{Backspace}')
      expect(input().value).toBe('48')
    })

    it('the arrow keys and Home move the highlighted box', async () => {
      render(<Harness initial="481" />)
      focusInput()
      act(() => input().setSelectionRange(3, 3))
      fireEvent.select(input())
      const active = () => boxes().findIndex((b) => b.dataset.active === 'true')
      expect(active()).toBe(3)
      await userEvent.keyboard('{ArrowLeft}')
      expect(active()).toBe(2)
      await userEvent.keyboard('{ArrowLeft}{ArrowLeft}')
      expect(active()).toBe(0)
      await userEvent.keyboard('{End}')
      expect(active()).toBe(3)
      await userEvent.keyboard('{Home}')
      expect(active()).toBe(0)
    })

    it('highlights the last box, not a seventh, when full', () => {
      render(<Harness initial="481209" />)
      focusInput()
      expect(boxes().findIndex((b) => b.dataset.active === 'true')).toBe(5)
    })

    it('does not highlight any box when the input has no focus', () => {
      render(<Harness initial="48" />)
      expect(boxes().some((b) => b.dataset.active === 'true')).toBe(false)
    })
  })

  describe('states', () => {
    it('error: marks the field invalid and every box with the error colour', () => {
      render(<Harness initial="481209" status="error" describedBy="msg" />)
      expect(input()).toHaveAttribute('aria-invalid', 'true')
      expect(input()).toHaveAttribute('aria-describedby', 'msg')
      expect(boxes().every((b) => b.className.includes('field-danger'))).toBe(true)
    })

    it('error: one sideways jolt of the row, then still', () => {
      const { container } = render(<Harness initial="481209" status="error" errorKey={1} />)
      expect(container.querySelector('[data-otp-row]')).toHaveAttribute('data-jolt', 'true')
    })

    it('error under reduced motion: the colour changes with a fade and nothing moves', () => {
      updatePreferences({ motion: 'reduce' })
      try {
        const { container } = render(<Harness initial="481209" status="error" errorKey={1} />)
        const row = container.querySelector('[data-otp-row]')!
        expect(row).not.toHaveAttribute('data-jolt')
        expect(row.getAttribute('style') ?? '').not.toMatch(/translate|rotate/)
      } finally {
        updatePreferences({ motion: 'system' })
      }
    })

    it('a typed digit pops in: the box from .92, the digit from 6px below', async () => {
      render(<Harness />)
      await userEvent.type(input(), '4')
      const box = boxes()[0]!
      expect(box.getAttribute('style') ?? '').toMatch(/scale/)
      expect(box.querySelector('[data-otp-digit]')!.getAttribute('style') ?? '').toMatch(/translateY\(6px\)|translate/)
    })

    it('a typed digit under reduced motion just appears: no transform', async () => {
      updatePreferences({ motion: 'reduce' })
      try {
        render(<Harness />)
        await userEvent.type(input(), '4')
        expect(boxes()[0]!.getAttribute('style') ?? '').not.toMatch(/scale|translate/)
        expect(boxes()[0]!.textContent).toBe('4')
      } finally {
        updatePreferences({ motion: 'system' })
      }
    })

    it('pending: keeps the code, ignores typing and says it is busy', async () => {
      const onChange = vi.fn()
      render(<Harness initial="481209" status="pending" onChange={onChange} />)
      expect(input()).toHaveAttribute('aria-busy', 'true')
      expect(input()).toHaveAttribute('readonly')
      await userEvent.type(input(), '7')
      expect(onChange).not.toHaveBeenCalled()
      expect(input().value).toBe('481209')
    })

    it('success: the boxes settle and the input stops taking input', () => {
      render(<Harness initial="481209" status="success" />)
      expect(input()).toHaveAttribute('readonly')
      expect(boxes().every((b) => b.dataset.status === 'success')).toBe(true)
    })

    it('success: the boxes gather into a check badge, which is decoration for the message beside it', () => {
      const { container } = render(<Harness initial="481209" status="success" />)
      const badge = container.querySelector('[data-otp-badge]')!
      expect(badge).toHaveAttribute('aria-hidden', 'true')
      expect(badge.querySelector('svg')).not.toBeNull()
    })

    it('no badge before the code is right', () => {
      const { container } = render(<Harness initial="481209" status="pending" />)
      expect(container.querySelector('[data-otp-badge]')).toBeNull()
    })

    it('disabled: the input is disabled', () => {
      render(<Harness disabled />)
      expect(input()).toBeDisabled()
    })

    it('idle boxes carry no error colour', () => {
      render(<Harness initial="481" />)
      expect(boxes().some((b) => b.className.includes('field-danger'))).toBe(false)
    })
  })

  it('takes focus on mount when asked', () => {
    render(<Harness autoFocus />)
    expect(input()).toHaveFocus()
  })

  it('a value set by the parent (cleared after a wrong code) empties the boxes without submitting', () => {
    const onComplete = vi.fn()
    const { rerender } = render(
      <OtpInput label="Mã xác minh" value="481209" onChange={() => {}} onComplete={onComplete} />,
    )
    rerender(<OtpInput label="Mã xác minh" value="" onChange={() => {}} onComplete={onComplete} />)
    expect(boxes().map((b) => b.textContent)).toEqual(['', '', '', '', '', ''])
    expect(onComplete).not.toHaveBeenCalled()
  })
})
