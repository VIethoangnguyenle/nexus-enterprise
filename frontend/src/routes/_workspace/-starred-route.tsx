import { StarredView } from '../../components/spaces/StarredView'
import { useShortcutScreen } from '../../hooks/useShortcutScreen'

/** Đã gắn sao: the main pane on a desktop, a full screen with Quay lại on a phone. */
export function StarredRoute() {
  const s = useShortcutScreen()
  return (
    <StarredView
      workspaceId={s.workspaceId}
      starredConversations={s.starred}
      conversationOf={s.conversationOf}
      onOpen={s.open}
      onOpenConversation={s.openConversation}
      onHome={s.home}
      onBack={s.back}
      hrefOf={s.hrefOf}
      phone={s.phone}
    />
  )
}
