import { useSyncExternalStore, type ReactNode } from 'react'
import type { Room } from 'livekit-client'
import { Room as FakeRoom } from '../../test/fake-livekit'
import { CallRoom, type CallSnapshot } from '../../lib/call-room'

/** A real `CallRoom` over the fake LiveKit room, so tests drive it with room events. */
export function makeCallRoom() {
  const room = new FakeRoom()
  const callRoom = new CallRoom(room as unknown as Room, { selfId: 'me', selfName: 'Lê Thị Hoa' })
  return { room, callRoom }
}

/** Re-renders `children` whenever the room's snapshot changes, as the provider does for the screens. */
export function WithSnapshot({ callRoom, children }: { callRoom: CallRoom; children: (s: CallSnapshot) => ReactNode }) {
  const snap = useSyncExternalStore(callRoom.subscribe, callRoom.getSnapshot)
  return <>{children(snap)}</>
}
