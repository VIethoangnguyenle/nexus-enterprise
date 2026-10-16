import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/react'

const bootstrapSession = vi.fn(() => Promise.resolve())
vi.mock('../api/client', () => ({ bootstrapSession: () => bootstrapSession() }))
vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router')
  return { ...actual, Outlet: () => <div data-testid="outlet" /> }
})

const { Route, resetSessionRestore } = await import('./__root')

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  resetSessionRestore()
})

describe('root route session restore', () => {
  it('starts the refresh from the router, before anything renders, and returns without waiting', () => {
    const result = (Route.options.beforeLoad as () => unknown)()
    expect(result).toBeUndefined()
    expect(bootstrapSession).toHaveBeenCalledTimes(1)
  })

  it('does not start a second refresh when the component mounts afterwards', () => {
    ;(Route.options.beforeLoad as () => unknown)()
    const Root = Route.options.component as () => React.ReactElement
    render(<Root />)
    render(<Root />)
    expect(bootstrapSession).toHaveBeenCalledTimes(1)
  })
})
