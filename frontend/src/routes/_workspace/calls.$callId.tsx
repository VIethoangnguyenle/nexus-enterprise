import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router'
import { validateWorkspaceSearch } from '../../lib/workspace'

/**
 * `answer` says how a call was picked up from a ring or a push: with the camera
 * (`video`) or without (`audio`). It skips the check before joining.
 */
export interface CallSearch {
  ws?: string
  answer?: 'video' | 'audio'
}

export function validateCallSearch(search: Record<string, unknown>): CallSearch {
  const out: CallSearch = { ...validateWorkspaceSearch(search) }
  if (search.answer === 'video' || search.answer === 'audio') out.answer = search.answer
  return out
}

// The call screen and the SDK behind it load only when a call is opened.
export const Route = createFileRoute('/_workspace/calls/$callId')({
  validateSearch: validateCallSearch,
  component: lazyRouteComponent(() => import('../../components/calls/CallScreen'), 'CallRoute'),
})
