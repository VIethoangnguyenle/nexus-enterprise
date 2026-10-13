import { describe, expect, it } from 'vitest'
import { chatTabOf } from '../lib/nav-destinations'
import { Route } from './_workspace/channels.index'

const validate = Route.options.validateSearch as (s: Record<string, unknown>) => { ws?: string; tab?: string }

describe('/channels search', () => {
  it('opens Trang chủ when the address has no tab', () => {
    expect(chatTabOf(validate({}))).toBe('home')
  })

  it('keeps a tab it knows and the workspace beside it', () => {
    expect(validate({ tab: 'sections', ws: 'w1' })).toEqual({ ws: 'w1', tab: 'sections' })
  })

  it('treats an unknown tab as Trang chủ', () => {
    expect(chatTabOf(validate({ tab: 'bogus' }))).toBe('home')
  })
})
