import { describe, expect, it } from 'vitest'
import { validateCallSearch } from './_workspace/calls.$callId'

describe('call route search', () => {
  it('keeps answer=video|audio and the workspace', () => {
    expect(validateCallSearch({ ws: 'w1', answer: 'video' })).toEqual({ ws: 'w1', answer: 'video' })
    expect(validateCallSearch({ answer: 'audio' })).toEqual({ answer: 'audio' })
  })

  it('drops anything else', () => {
    expect(validateCallSearch({ answer: 'screen' })).toEqual({})
    expect(validateCallSearch({ answer: 1 as unknown as string })).toEqual({})
    expect(validateCallSearch({})).toEqual({})
  })
})
