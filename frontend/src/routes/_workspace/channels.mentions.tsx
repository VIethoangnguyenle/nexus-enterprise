import { createFileRoute } from '@tanstack/react-router'
import { MentionsView } from '../../components/spaces/MentionsView'
import { useShortcutScreen } from '../../hooks/useShortcutScreen'
import { validateWorkspaceSearch } from '../../lib/workspace'

export const Route = createFileRoute('/_workspace/channels/mentions')({
  validateSearch: validateWorkspaceSearch,
  component: MentionsRoute,
})

/** Được nhắc tới: the main pane on a desktop, a full screen with Quay lại on a phone. */
export function MentionsRoute() {
  const s = useShortcutScreen()
  return (
    <MentionsView
      workspaceId={s.workspaceId}
      conversationOf={s.conversationOf}
      onOpen={s.open}
      onHome={s.home}
      onBack={s.back}
      hrefOf={s.hrefOf}
      phone={s.phone}
    />
  )
}
