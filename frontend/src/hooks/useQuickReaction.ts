import { useServerSettings } from './useServerSettings'

/** The double-tap reaction when the person never chose one, or the settings could not be read. */
export const DEFAULT_QUICK_REACTION = '👍'

/**
 * The reaction a double-tap on a message toggles: the person's Chat setting `double_tap` in this workspace
 * (Cài đặt Chat, "Cảm xúc khi chạm đúp"). An empty string is the person turning it off; a key never set, a
 * settings read that failed, or no workspace yet falls back to 👍 (DESIGN.md §6 "Vuốt"). Until the first answer
 * there is none, so a person who turned it off never gets a 👍 from a double-tap made while it loads.
 */
export function useQuickReaction(workspaceId: string | undefined): string {
  const { settings, isLoading } = useServerSettings('chat', workspaceId ?? '')
  if (isLoading) return ''
  return settings.double_tap ?? DEFAULT_QUICK_REACTION
}
