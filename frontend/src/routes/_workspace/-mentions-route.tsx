import { MentionsView } from '../../components/spaces/MentionsView'
import { useShortcutScreen } from '../../hooks/useShortcutScreen'

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
