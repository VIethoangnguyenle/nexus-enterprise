import { describe, expect, it } from 'vitest'
import { deviceOptions } from './call-devices'

const dev = (kind: MediaDeviceKind, deviceId: string, label: string) => ({ kind, deviceId, label }) as MediaDeviceInfo

describe('deviceOptions', () => {
  it('keeps one kind and uses the operating system name', () => {
    const out = deviceOptions([
      dev('audioinput', 'a1', 'Micro MacBook Air (mặc định)'),
      dev('videoinput', 'v1', 'Camera FaceTime HD'),
    ], 'audioinput')
    expect(out).toEqual([{ id: 'a1', label: 'Micro MacBook Air (mặc định)' }])
  })

  it('numbers a device whose name is withheld, instead of showing its id', () => {
    const out = deviceOptions([dev('videoinput', 'x9f3', ''), dev('videoinput', 'y7c1', '')], 'videoinput')
    expect(out.map((o) => o.label)).toEqual(['Camera 1', 'Camera 2'])
    expect(JSON.stringify(out.map((o) => o.label))).not.toContain('x9f3')
  })
})
