import { describe, expect, it } from 'vitest'
import { isConversationPath } from './workspace'

describe('isConversationPath', () => {
  it('is one open conversation, not the list nor another screen', () => {
    expect(isConversationPath('/channels/c-1')).toBe(true)
    expect(isConversationPath('/channels/c-1/')).toBe(true)
    expect(isConversationPath('/channels')).toBe(false)
    expect(isConversationPath('/channels/')).toBe(false)
    expect(isConversationPath('/drive')).toBe(false)
    expect(isConversationPath('/documents/d-1')).toBe(false)
  })
})
