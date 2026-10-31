import { useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { barModules, defaultBar, resolveBar, type BarModule, type BarModuleId } from '../lib/nav-destinations'
import { unreadByLevel } from '../lib/conversations'
import { approvalPendingOptions } from './useApproval'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useUnreadCounts } from './useMessaging'
import { useUnreadNotificationCount } from './useNotifications'
import { useServerSettings } from './useServerSettings'
import { useWorkspaceType } from './useWorkspaceType'

/**
 * The bottom bar of the phone for the open workspace: the four slots this person chose (the `mobile.bar`
 * setting of the workspace, kept on the server), or the defaults of its kind when none is set or the
 * choice names something the workspace does not offer. `ready` is false until the kind of workspace and
 * the setting are known, so the bar never draws one set of slots and then another.
 */
export function useModuleBar() {
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal, isLoading: kindLoading } = useWorkspaceType()
  const { settings, isLoading: settingLoading, save } = useServerSettings('workspace', workspaceId)

  const modules = useMemo(() => barModules(isPersonal), [isPersonal])
  const ids = useMemo(() => resolveBar(settings['mobile.bar'], isPersonal), [settings, isPersonal])
  const slots = useMemo(() => ids.map((id) => modules.find((m) => m.id === id)!), [ids, modules])
  const rest = useMemo(() => modules.filter((m) => !ids.includes(m.id)), [ids, modules])

  const saveBar = useCallback((next: BarModuleId[]) => save({ 'mobile.bar': next }), [save])
  const resetBar = useCallback(() => saveBar(defaultBar(isPersonal)), [saveBar, isPersonal])

  return {
    ready: !kindLoading && !settingLoading,
    isPersonal,
    modules,
    slots: slots as BarModule[],
    rest,
    ids,
    saveBar,
    resetBar,
    defaults: defaultBar(isPersonal),
  }
}

/** What the number on each module counts; the same rules as the sidebar's. `enabled` false asks for nothing. */
export function useModuleNews(enabled: boolean, isPersonal: boolean): Record<BarModuleId, number> {
  // A muted conversation adds only the times someone named the reader.
  const chat = unreadByLevel(useUnreadCounts().data?.channels)
  const approval = useQuery({ ...approvalPendingOptions(), enabled: enabled && !isPersonal }).data?.total ?? 0
  const notifications = useUnreadNotificationCount()
  return { chat, approval, notifications, calendar: 0, drive: 0, assets: 0, contacts: 0 }
}
