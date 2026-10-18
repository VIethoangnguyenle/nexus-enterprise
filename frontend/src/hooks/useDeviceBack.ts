import { useCallback } from 'react'
import { useRouter, useRouterState } from '@tanstack/react-router'
import { isDeviceBack, markOwnBack, markOwnStep } from '../lib/device-back'

/**
 * Was the step that put this screen on show the device's own back? Then the device has drawn the move
 * already and the screen must appear at once, with no entrance or exit animation of its own.
 * `ownBack` is called just before the app's own `history.back()`; `ownStep` before a replace made by the app.
 */
export function useDeviceBack() {
  const router = useRouter()
  const index = useRouterState({ select: (s) => (s.location.state as unknown as { __TSR_index?: number }).__TSR_index ?? 0 })
  const byDevice = isDeviceBack(router.history, index)
  const ownBack = useCallback(() => markOwnBack(router.history), [router])
  const ownStep = useCallback(() => markOwnStep(router.history), [router])
  return { byDevice, ownBack, ownStep }
}
