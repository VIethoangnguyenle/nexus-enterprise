import { expect, type Mock } from 'vitest'

interface RouteArg { to: string; params: { callId: string }; search: (prev: { ws?: string }) => unknown }

/**
 * Asserts that `navigate` was sent to a call's screen: which call, how it is
 * picked up (`answer`), and that the workspace in the address is carried along.
 */
export function expectCallRoute(navigate: Mock, callId: string, answer?: 'audio' | 'video') {
  const hit = navigate.mock.calls
    .map((c) => c[0] as RouteArg)
    .find((a) => a.to === '/calls/$callId' && a.params.callId === callId)
  expect(hit, `no navigation to call ${callId}`).toBeDefined()
  expect(typeof hit!.search).toBe('function')
  expect(hit!.search({ ws: 'w1' })).toEqual(answer ? { ws: 'w1', answer } : { ws: 'w1' })
}
