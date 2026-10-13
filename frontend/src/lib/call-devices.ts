export type DeviceKind = 'audioinput' | 'videoinput' | 'audiooutput'

export interface DeviceOption { id: string; label: string }

const FALLBACK: Record<DeviceKind, string> = { audioinput: 'Micro', videoinput: 'Camera', audiooutput: 'Loa' }

/**
 * Devices as the person should see them: the name the operating system gave,
 * never the device id. A device whose name the browser withholds (before the
 * permission is given) is numbered instead.
 */
export function deviceOptions(devices: MediaDeviceInfo[], kind: DeviceKind): DeviceOption[] {
  return devices
    .filter((d) => d.kind === kind)
    .map((d, i) => ({ id: d.deviceId, label: d.label?.trim() || `${FALLBACK[kind]} ${i + 1}` }))
}

/** Lists the devices of one kind through the SDK (loaded here, not in the main bundle). */
export async function listDevices(kind: DeviceKind, requestPermissions: boolean): Promise<DeviceOption[]> {
  try {
    const { Room } = await import('livekit-client')
    return deviceOptions(await Room.getLocalDevices(kind, requestPermissions), kind)
  } catch {
    return []
  }
}
