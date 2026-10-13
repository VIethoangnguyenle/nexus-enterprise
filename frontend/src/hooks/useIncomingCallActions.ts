import { useNavigate } from '@tanstack/react-router'
import { callsApi, type CallKind } from '../api/calls'
import { ApiError } from '../api/client'
import { callRoute } from '../lib/call-nav'
import { explain } from '../lib/errors'
import type { IncomingCallInfo } from '../stores/incoming-call.store'
import { toast } from '../components/primitives'

/**
 * What the buttons of an incoming call do, for the desktop card and the phone
 * screen alike: decline, answer with or without the camera, join an
 * invitation, dismiss. Each one also takes the ring off the screen.
 */
export function useIncomingCallActions(incoming: IncomingCallInfo, onClear: (callId: string) => void) {
  const navigate = useNavigate()
  const { callId, kind } = incoming

  const decline = async () => {
    onClear(callId)
    try {
      await callsApi.decline(callId)
    } catch (err) {
      // A call that had already stopped ringing (answered, cancelled, missed) answers 409 call_ended: nothing to tell.
      const gone = err instanceof ApiError && (err.body as { code?: unknown } | undefined)?.code === 'call_ended'
      if (!gone) toast.error(explain(err, 'từ chối cuộc gọi'))
    }
  }

  const answer = (how: CallKind) => {
    void navigate(callRoute(callId, how))
    onClear(callId)
  }

  return {
    decline: () => void decline(),
    answer,
    /** Joining an invitation: with the camera if it is a video call. */
    join: () => answer(kind),
    dismiss: () => onClear(callId),
  }
}
