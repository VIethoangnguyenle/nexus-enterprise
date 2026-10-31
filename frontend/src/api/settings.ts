import { apiFetch } from './client'

/** The bottom-bar slots a person picked for one workspace: four distinct module ids. */
export interface WorkspaceSettings {
  'mobile.bar'?: string[]
}

/** Chat's options of one person in one workspace (messaging). A PUT changes only the keys sent. */
export interface ChatSettings {
  enter_to_send?: boolean
  link_preview?: boolean
  /** Where Chat opens on a phone: every conversation, or the person's sections. */
  open_at?: 'all' | 'sections'
  sort?: 'activity' | 'name'
  /** The reaction a double tap adds; "" is off. */
  double_tap?: string
  /** The level a new group starts at for this person; null is the system default. */
  default_level?: 'all' | 'main' | 'off' | null
}

/** Tài liệu's options (drive). A PUT replaces every field. */
export interface DriveSettings {
  view?: 'auto' | 'grid' | 'list'
  sort?: 'modified' | 'name' | 'size'
  folders_first?: boolean
  cellular_previews?: boolean
  notify_shared?: boolean
}

/** Tài sản's options (asset). A PUT replaces every field. */
export interface AssetSettings {
  default_tab?: 'overview' | 'inventory' | 'requests'
  sort?: 'name' | 'created' | 'updated' | 'status'
  notify_requests?: boolean
  notify_maintenance?: boolean
}

/**
 * Lịch's options (calendar, `calendar_user_settings`), in the calendar service's
 * own camelCase. A PUT replaces every field, the zone and working hours included.
 */
export interface CalendarSettings {
  tzid?: string
  workStart?: string
  workEnd?: string
  defaultReminder?: number
  defaultView?: 'agenda' | 'day' | 'week' | 'month'
  /** 0 Sunday, 1 Monday. */
  weekStart?: 0 | 1
  visibleCalendars?: string[]
  defaultDuration?: number
  notifyInvites?: boolean
  notifyChanges?: boolean
  notifyReminders?: boolean
}

/** Phê duyệt's options (approval, organization workspaces only). A PUT replaces every field. */
export interface ApprovalSettings {
  default_tab?: 'pending' | 'approved' | 'rejected'
  sort?: 'newest' | 'oldest'
  notify_pending?: boolean
  notify_result?: boolean
}

/** The services that keep person-level settings, each at its own endpoint. */
export interface SettingsByService {
  workspace: WorkspaceSettings
  chat: ChatSettings
  drive: DriveSettings
  asset: AssetSettings
  calendar: CalendarSettings
  approval: ApprovalSettings
}
export type SettingsService = keyof SettingsByService

interface Endpoint<S> {
  /**
   * The path of one scope's settings (a workspace id); GET reads it, PUT writes it.
   * Every service's PUT is a merge: the keys a body leaves out keep their stored
   * value, so a body carries only the change.
   */
  path: (scopeId: string) => string
  /** The answer's body as settings, when the service wraps them. */
  decode?: (raw: unknown) => S
  /** The body of a PUT, when the service wraps the settings. */
  encode?: (settings: S) => unknown
}

const ws = (id: string) => encodeURIComponent(id)

/** The workspace service answers and takes `{"settings": {key: value}}`. */
const unwrapSettings = (raw: unknown): WorkspaceSettings => {
  const inner = (raw as { settings?: unknown } | null)?.settings
  return inner && typeof inner === 'object' ? (inner as WorkspaceSettings) : {}
}

/**
 * Where each service keeps its settings. Another service joins by adding its
 * path here and its shape to `SettingsByService`; nothing else changes.
 */
export const settingsEndpoints: { [S in SettingsService]: Endpoint<SettingsByService[S]> } = {
  workspace: {
    path: (id) => `/workspaces/${ws(id)}/settings`,
    decode: unwrapSettings,
    encode: (settings) => ({ settings }),
  },
  chat: { path: (id) => `/workspaces/${ws(id)}/chat-settings` },
  drive: { path: (id) => `/drive/settings?workspace_id=${ws(id)}` },
  asset: { path: (id) => `/workspaces/${ws(id)}/asset-settings` },
  calendar: { path: (id) => `/workspaces/${ws(id)}/calendar/settings` },
  // Approval keeps them per organization, the one of the session.
  approval: { path: () => `/approval/settings` },
}

export const settingsApi = {
  get: async <S extends SettingsService>(service: S, scopeId: string): Promise<SettingsByService[S]> => {
    const e = settingsEndpoints[service] as Endpoint<SettingsByService[S]>
    const raw = await apiFetch<unknown>(e.path(scopeId))
    return e.decode ? e.decode(raw) : (raw as SettingsByService[S])
  },
  put: async <S extends SettingsService>(service: S, scopeId: string, body: SettingsByService[S]): Promise<SettingsByService[S]> => {
    const e = settingsEndpoints[service] as Endpoint<SettingsByService[S]>
    const raw = await apiFetch<unknown>(e.path(scopeId), { method: 'PUT', body: JSON.stringify(e.encode ? e.encode(body) : body) })
    return e.decode ? e.decode(raw) : (raw as SettingsByService[S])
  },
}
