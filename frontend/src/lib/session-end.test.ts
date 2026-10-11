import { describe, it, expect, vi } from 'vitest'
import { onSessionEnd, endSession } from './session-end'

describe('session end', () => {
  it('runs every registered reset, even after one throws', () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const a = vi.fn(() => { throw new Error('boom') })
    const b = vi.fn()
    const offA = onSessionEnd(a)
    const offB = onSessionEnd(b)
    endSession()
    expect(a).toHaveBeenCalledOnce()
    expect(b).toHaveBeenCalledOnce()
    offA(); offB(); err.mockRestore()
  })

  it('stops calling a reset once unregistered', () => {
    const f = vi.fn()
    onSessionEnd(f)()
    endSession()
    expect(f).not.toHaveBeenCalled()
  })
})
