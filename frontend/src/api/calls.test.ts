import { describe, expect, it } from 'vitest'
import { asCallKind } from './calls'

describe('asCallKind', () => {
  it('lets audio and video through and nothing else', () => {
    expect(asCallKind('audio')).toBe('audio')
    expect(asCallKind('video')).toBe('video')
    for (const bad of ['', 'hologram', 'AUDIO', undefined, null, 3]) expect(asCallKind(bad)).toBeNull()
  })
})
