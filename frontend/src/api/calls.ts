import { apiFetch, ApiError } from './client'

/** `{ seconds, nanos }` as the backend serialises a protobuf Timestamp; read it with `toDate` / `toMillis`. */
export interface WireTimestamp { seconds: number | string; nanos?: number }

export type CallKind = 'audio' | 'video'
/** Reads a wire value as a call kind; anything but audio or video is not one. */
export function asCallKind(v: unknown): CallKind | null {
  return v === 'audio' || v === 'video' ? v : null
}

export type CallStatus = 'ringing' | 'active' | 'ended' | 'missed' | 'declined' | 'cancelled'

export interface CallParticipant {
  user_id: string
  display_name?: string
  avatar_url?: string
  joined_at?: WireTimestamp
}

/** Absent fields are omitted by the server, so everything optional here may be missing. */
export interface Call {
  id: string
  /** Absent for the call of a calendar event, which is in no conversation. */
  channel_id?: string
  /** The event a calendar event's call belongs to; the screen names the call after it. */
  calendar_event_id?: string
  kind: CallKind
  status: CallStatus
  started_by?: string
  started_by_name?: string
  started_by_avatar_url?: string
  message_id?: string
  created_at?: WireTimestamp
  started_at?: WireTimestamp
  ended_at?: WireTimestamp
  ring_expires_at?: WireTimestamp
  participants?: CallParticipant[]
  taker_count?: number
}

/** A call plus what it takes to enter its room: a LiveKit access token (10 minutes) and the signalling URL. */
export interface CallJoin {
  call: Call
  token: string
  url: string
}

/** The most people one call holds. */
export const MAX_CALL_PEOPLE = 20

export const callsApi = {
  /** 201 for a new call, 200 when the conversation already has an open one and this joins it. */
  start: (channelId: string, kind: CallKind) =>
    apiFetch<CallJoin>(`/channels/${channelId}/calls`, { method: 'POST', body: JSON.stringify({ kind }) }),
  /** A fresh entry ticket: on joining, and again when a reconnect fails because the first one expired. */
  token: (callId: string) => apiFetch<CallJoin>(`/calls/${callId}/token`, { method: 'POST' }),
  get: (callId: string) => apiFetch<Call>(`/calls/${callId}`),
  /** The conversation's ringing or running call, or null. */
  open: (channelId: string) =>
    apiFetch<{ call?: Call | null }>(`/channels/${channelId}/call`).then((r) => r.call ?? null),
  decline: (callId: string) => apiFetch<Call>(`/calls/${callId}/decline`, { method: 'POST' }),
  cancel: (callId: string) => apiFetch<Call>(`/calls/${callId}/cancel`, { method: 'POST' }),
  invite: (callId: string, userIds: string[]) =>
    apiFetch<void>(`/calls/${callId}/invite`, { method: 'POST', body: JSON.stringify({ user_ids: userIds }) }),
}

/** Why a ticket was refused, in the terms the call screens speak. */
export type CallRefusal = 'call_full' | 'call_ended' | 'unavailable' | 'forbidden' | 'unreachable'

/** Reads the refusal off a failed call request: the server's `code` where it gave one, else the status. */
export function refusalOf(err: unknown): CallRefusal {
  if (!(err instanceof ApiError)) return 'unreachable'
  const code = (err.body as { code?: unknown } | undefined)?.code
  if (code === 'call_full') return 'call_full'
  if (code === 'call_ended') return 'call_ended'
  if (code === 'calls_unavailable' || err.status === 503) return 'unavailable'
  if (err.status === 403) return 'forbidden'
  if (err.status === 404 || err.status === 410) return 'call_ended'
  return 'unreachable'
}
