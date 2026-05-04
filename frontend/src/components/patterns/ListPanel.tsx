import { Link } from '@tanstack/react-router'
import { useUiStore } from '../../stores/ui.store'
import { ChatList } from './ChatList'
import { FileText, Settings as SettingsIcon, HardDrive, LayoutDashboard, ClipboardList, Tag, FileEdit } from 'lucide-react'
import { Heading } from '../primitives'

interface ListPanelProps {
  workspaceId: string
}

/** Contextual list panel — content changes based on active module. Width controlled by parent. */
export function ListPanel({ workspaceId }: ListPanelProps) {
  const activeModule = useUiStore((s) => s.activeModule)

  /* Contacts/Drive use their own full-width layout — no list panel needed */
  if (activeModule === 'contacts' || activeModule === 'drive') {
    return null
  }

  return (
    <div className="flex-shrink-0 bg-surface-bright border-r border-outline-variant/30
      flex flex-col overflow-hidden h-full">
      {activeModule === 'messaging' && <ChatList workspaceId={workspaceId} />}
      {activeModule === 'documents' && <DocumentList workspaceId={workspaceId} />}
      {activeModule === 'assets' && <AssetNav />}
      {activeModule === 'settings' && <SettingsNav />}
    </div>
  )
}

function DocumentList({ workspaceId: _wsId }: { workspaceId: string }) {
  return (
    <>
      <div className="flex items-center px-3 py-2 border-b border-outline-variant/20">
        <Heading as="h4">Documents</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        <div className="px-4 py-6 text-center">
          <span className="text-caption text-on-surface-variant">Documents open in Docs tab</span>
        </div>
      </div>
    </>
  )
}

function AssetNav() {
  const links = [
    { to: '/assets/dashboard', icon: <LayoutDashboard size={16} className="flex-shrink-0" />, label: 'Dashboard' },
    { to: '/assets/list', icon: <ClipboardList size={16} className="flex-shrink-0" />, label: 'All Assets' },
    { to: '/assets/types', icon: <Tag size={16} className="flex-shrink-0" />, label: 'Asset Types' },
    { to: '/assets/requests', icon: <FileEdit size={16} className="flex-shrink-0" />, label: 'Requests' },
  ] as const

  return (
    <>
      <div className="flex items-center px-3 py-2 border-b border-outline-variant">
        <Heading as="h4">Assets</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        {links.map((l) => (
          <Link
            key={l.to}
            to={l.to}
            className="flex items-center gap-2 px-4 py-2 text-small text-on-surface-variant
              hover:bg-surface-container hover:text-on-surface transition-colors duration-fast
              no-underline rounded-md mx-1 focus-ring"
            activeProps={{
              className:
                'flex items-center gap-2 px-4 py-2 text-small-ui bg-primary/8 text-primary no-underline rounded-md mx-1',
            }}
          >
            {l.icon}
            <span className="truncate">{l.label}</span>
          </Link>
        ))}
      </div>
    </>
  )
}

function SettingsNav() {
  return (
    <>
      <div className="flex items-center px-3 py-2 border-b border-outline-variant">
        <Heading as="h4">Settings</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        <Link
          to="/settings"
          className="flex items-center gap-2 px-4 py-2 text-small text-on-surface-variant
            hover:bg-surface-container hover:text-on-surface transition-colors duration-fast
            no-underline rounded-md mx-1 focus-ring"
          activeProps={{
            className:
              'flex items-center gap-2 px-4 py-2 text-small-ui bg-primary/8 text-primary no-underline rounded-md mx-1',
          }}
        >
          <SettingsIcon size={16} className="flex-shrink-0" />
          <span>Workspace Settings</span>
        </Link>
      </div>
    </>
  )
}
