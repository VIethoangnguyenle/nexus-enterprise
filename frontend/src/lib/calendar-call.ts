/** Minutes before the start from which the event's video call can be joined (DESIGN.md §6, Popover chi tiết). */
export const CALL_OPENS_BEFORE_MIN = 15

export type CallWindow = 'early' | 'open' | 'over'

/** Whether the call of an event can be joined now: from 15 minutes before it starts until it ends. */
export function callWindow(start: Date, end: Date, now: Date): CallWindow {
  if (now.getTime() >= end.getTime()) return 'over'
  return now.getTime() >= start.getTime() - CALL_OPENS_BEFORE_MIN * 60_000 ? 'open' : 'early'
}
