import { useCallback } from 'react'
import { useCanGoBack, useNavigate, useRouter } from '@tanstack/react-router'

/**
 * The pages under Cài đặt (`/settings/<section>`). Tôi is `/settings` itself on a phone; each module's page is
 * `/settings/<module>`, one page whether it is opened from Tôi or from the module's ⋯ (DESIGN.md §6 "Cài đặt").
 */
export const SETTINGS_SECTIONS = [
  'account', 'org-profile', 'notifications', 'appearance', 'sessions',
  'workspace', 'chat', 'calendar', 'drive', 'approval', 'assets',
] as const
export type SettingsSection = (typeof SETTINGS_SECTIONS)[number]

export const isSettingsSection = (v: unknown): v is SettingsSection =>
  (SETTINGS_SECTIONS as readonly unknown[]).includes(v)

/** Only the workspace follows to another settings page. */
const keepWorkspace = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})

/** Opening a settings page, and going back from one to where it was opened (Tôi when there is no history). */
export function useSettingsNav() {
  const navigate = useNavigate()
  const router = useRouter()
  const canGoBack = useCanGoBack()

  const open = useCallback(
    (section: SettingsSection) => void navigate({ to: '/settings/$section', params: { section }, search: keepWorkspace }),
    [navigate],
  )
  /** Back where the page was opened from; with no history, to `fallback` (Tôi when none). */
  const back = useCallback((fallback?: SettingsSection) => {
    if (canGoBack) router.history.back()
    else if (fallback) open(fallback)
    else void navigate({ to: '/settings', search: keepWorkspace })
  }, [canGoBack, router, navigate, open])

  return { open, back }
}
