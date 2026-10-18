import type { ApprovalSettings, AssetSettings, CalendarSettings, ChatSettings, DriveSettings } from '../api/settings'

/**
 * What each service answers for a person who never saved its settings (the column defaults of its table).
 * A screen reading a setting uses these until the server answers, or when the read fails, so it never waits
 * on the settings to draw. They live apart from the settings pages so a module screen does not pull a page
 * into its chunk.
 */
export const CHAT_DEFAULTS: ChatSettings = {
  enter_to_send: false, link_preview: true, open_at: 'all', sort: 'activity', double_tap: '👍', default_level: null,
}

export const DRIVE_DEFAULTS: DriveSettings = {
  view: 'auto', sort: 'modified', folders_first: true, cellular_previews: true, notify_shared: true,
}

export const ASSET_DEFAULTS: AssetSettings = { default_tab: 'overview', sort: 'name', notify_requests: true, notify_maintenance: true }

export const CALENDAR_DEFAULTS: CalendarSettings = {
  tzid: 'Asia/Ho_Chi_Minh', workStart: '08:00', workEnd: '17:30', defaultReminder: 10,
  // An empty list is every calendar.
  defaultView: 'agenda', weekStart: 1, visibleCalendars: [], defaultDuration: 30,
  notifyInvites: true, notifyChanges: true, notifyReminders: true,
}

export const APPROVAL_DEFAULTS: ApprovalSettings = { default_tab: 'pending', sort: 'newest', notify_pending: true, notify_result: true }
