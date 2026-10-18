import { describe, expect, it } from 'vitest'
import { callRoute } from './call-nav'

describe('callRoute', () => {
  it('keeps the open workspace when the call names none', () => {
    expect(callRoute('c1', 'audio').search({ ws: 'ws-b' })).toEqual({ ws: 'ws-b', answer: 'audio' })
    expect(callRoute('c1').search({ ws: 'ws-b' })).toEqual({ ws: 'ws-b' })
  })

  it('names the call\'s workspace when it has one, whichever workspace is open', () => {
    expect(callRoute('c1', 'video', 'ws-a').search({ ws: 'ws-b' })).toEqual({ ws: 'ws-a', answer: 'video' })
    expect(callRoute('c1', undefined, 'ws-a').search({})).toEqual({ ws: 'ws-a' })
  })
})
