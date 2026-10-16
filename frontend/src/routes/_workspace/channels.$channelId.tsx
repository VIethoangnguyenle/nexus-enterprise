import { createFileRoute } from '@tanstack/react-router'
import { ChannelRoute } from './-channel-route'
import { validateWorkspaceSearch } from '../../lib/workspace'

/**
 * Where to land in a conversation. `thread` is a root message: its panel opens.
 * `message` is the one to bring to the middle and flash, in the stream or, when
 * a thread is open, in its panel. Push links, Trang chủ, Được nhắc tới and Đã
 * gắn sao all open a conversation this way.
 */
export interface ChannelSearch {
  ws?: string
  thread?: string
  message?: string
}

/** Message ids are looked up in the page, so only id-shaped values are let through. */
const ID = /^[\w-]{1,64}$/

function idOf(value: unknown): string | undefined {
  return typeof value === 'string' && ID.test(value) ? value : undefined
}

export function validateChannelSearch(search: Record<string, unknown>): ChannelSearch {
  const out: ChannelSearch = { ...validateWorkspaceSearch(search) }
  const thread = idOf(search.thread)
  if (thread) out.thread = thread
  const message = idOf(search.message)
  if (message) out.message = message
  return out
}

export const Route = createFileRoute('/_workspace/channels/$channelId')({
  validateSearch: validateChannelSearch,
  component: ChannelRoute,
})
