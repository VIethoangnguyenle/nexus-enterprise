import { useUiStore } from '../../stores/ui.store'
import { ChatNavigator } from '../spaces/ChatNavigator'

interface ListPanelProps {
  workspaceId: string
}

/**
 * The shell's list panel. Only Tin nhắn draws one here: Tài liệu carries its
 * own (folders and Văn bản), and Danh bạ, Phê duyệt, Tài sản, Quản trị and Cài
 * đặt have none (DESIGN.md §5).
 */
export function ListPanel({ workspaceId: _workspaceId }: ListPanelProps) {
  const activeModule = useUiStore((s) => s.activeModule)

  if (activeModule !== 'messaging') return null

  /* Tin nhắn: the Google-Chat navigator sits on the page tone, no divider (DESIGN.md §4). Not clipped: the
   * search surface at the head of the column opens wider than the column. */
  return (
    <div className="flex-shrink-0 bg-base flex flex-col h-full">
      <ChatNavigator />
    </div>
  )
}
