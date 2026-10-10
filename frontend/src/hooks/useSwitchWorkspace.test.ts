import { describe, expect, it } from 'vitest'
import { moduleRootOf } from './useSwitchWorkspace'

describe('moduleRootOf', () => {
  it('keeps the module and drops what belongs to the old workspace', () => {
    expect(moduleRootOf('/drive')).toBe('/drive')
    expect(moduleRootOf('/channels/9f2c')).toBe('/channels')
    expect(moduleRootOf('/documents/abc')).toBe('/documents')
    expect(moduleRootOf('/admin/roles')).toBe('/admin')
    expect(moduleRootOf('/approval')).toBe('/approval')
  })
  it('falls back to Tin nhắn for anything else', () => {
    expect(moduleRootOf('/')).toBe('/channels')
    expect(moduleRootOf('/driveway')).toBe('/channels')
  })
})
