import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useUiStore } from '../../stores/ui.store'
import { useChannels } from '../../hooks/useMessaging'
import { useDocuments } from '../../hooks/useDocuments'
import { CreateChannelModal } from '../CreateChannelModal'
import { Heading, Text } from '../primitives'

interface ListPanelProps {
  workspaceId: string
}

/** Scrollable list panel showing items for the active module. */
export function ListPanel({ workspaceId }: ListPanelProps) {
  const activeModule = useUiStore((s) => s.activeModule)

  return (
    <div className="w-[280px] flex-shrink-0 bg-bg-secondary border-r border-border
      flex flex-col overflow-hidden animate-slide-left">
      {activeModule === 'messaging' && <ChannelList workspaceId={workspaceId} />}
      {activeModule === 'documents' && <DocumentList workspaceId={workspaceId} />}
      {activeModule === 'drive' && <DriveNav />}
      {activeModule === 'assets' && <AssetNav />}
      {activeModule === 'settings' && <SettingsNav />}
    </div>
  )
}

function ChannelList({ workspaceId }: { workspaceId: string }) {
  const { data } = useChannels(workspaceId)
  const channels = data?.channels || []
  const [showCreate, setShowCreate] = useState(false)

  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <Heading as="h4">Channels</Heading>
        <button
          onClick={() => setShowCreate(true)}
          className="w-6 h-6 flex items-center justify-center rounded text-text-muted
            hover:text-text-primary hover:bg-bg-hover transition-all cursor-pointer
            border-none bg-transparent text-sm"
          title="Create channel"
        >
          +
        </button>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        {channels.map(ch => (
          <Link
            key={ch.id}
            to="/channels/$channelId"
            params={{ channelId: ch.id }}
            className="flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary
              hover:bg-bg-hover hover:text-text-primary transition-colors no-underline"
            activeProps={{
              className: 'flex items-center gap-2.5 px-4 py-2 text-sm bg-bg-active text-accent-hover font-medium no-underline'
            }}
          >
            <span className="text-text-muted">#</span>
            <span className="truncate">{ch.name}</span>
          </Link>
        ))}
        {channels.length === 0 && (
          <div className="px-4 py-6 text-center">
            <Text variant="caption" muted>No channels yet</Text>
          </div>
        )}
      </div>
      {showCreate && <CreateChannelModal onClose={() => setShowCreate(false)} />}
    </>
  )
}

function DocumentList({ workspaceId }: { workspaceId: string }) {
  const { data } = useDocuments(workspaceId)
  const docs = data?.documents || []

  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <Heading as="h4">Documents</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        {docs.map(doc => (
          <Link
            key={doc.id}
            to="/documents"
            className="flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary
              hover:bg-bg-hover hover:text-text-primary transition-colors no-underline"
          >
            <span className="text-text-muted">📄</span>
            <span className="truncate">{doc.title}</span>
          </Link>
        ))}
        {docs.length === 0 && (
          <div className="px-4 py-6 text-center">
            <Text variant="caption" muted>No documents yet</Text>
          </div>
        )}
      </div>
    </>
  )
}

function DriveNav() {
  const links = [
    { to: '/drive', icon: '💾', label: 'My Drive' },
  ]

  return (
    <>
      <div className="flex items-center px-4 py-3 border-b border-border">
        <Heading as="h4">Drive</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        {links.map(l => (
          <Link
            key={l.to}
            to={l.to}
            className="flex items-center gap-2.5 px-4 py-2.5 text-sm text-text-secondary
              hover:bg-bg-hover hover:text-text-primary transition-colors no-underline"
            activeProps={{
              className: 'flex items-center gap-2.5 px-4 py-2.5 text-sm bg-bg-active text-accent-hover font-medium no-underline'
            }}
          >
            <span>{l.icon}</span>
            <span>{l.label}</span>
          </Link>
        ))}
      </div>
    </>
  )
}

function AssetNav() {
  const links = [
    { to: '/assets/dashboard', icon: '📊', label: 'Dashboard' },
    { to: '/assets/list', icon: '📋', label: 'All Assets' },
    { to: '/assets/types', icon: '🏷️', label: 'Asset Types' },
    { to: '/assets/requests', icon: '📝', label: 'Requests' },
  ]

  return (
    <>
      <div className="flex items-center px-4 py-3 border-b border-border">
        <Heading as="h4">Assets</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        {links.map(l => (
          <Link
            key={l.to}
            to={l.to}
            className="flex items-center gap-2.5 px-4 py-2.5 text-sm text-text-secondary
              hover:bg-bg-hover hover:text-text-primary transition-colors no-underline"
            activeProps={{
              className: 'flex items-center gap-2.5 px-4 py-2.5 text-sm bg-bg-active text-accent-hover font-medium no-underline'
            }}
          >
            <span>{l.icon}</span>
            <span>{l.label}</span>
          </Link>
        ))}
      </div>
    </>
  )
}

function SettingsNav() {
  return (
    <>
      <div className="flex items-center px-4 py-3 border-b border-border">
        <Heading as="h4">Settings</Heading>
      </div>
      <div className="flex-1 overflow-y-auto py-1">
        <Link
          to="/settings"
          className="flex items-center gap-2.5 px-4 py-2.5 text-sm text-text-secondary
            hover:bg-bg-hover hover:text-text-primary transition-colors no-underline"
          activeProps={{
            className: 'flex items-center gap-2.5 px-4 py-2.5 text-sm bg-bg-active text-accent-hover font-medium no-underline'
          }}
        >
          <span>⚙️</span>
          <span>Workspace Settings</span>
        </Link>
      </div>
    </>
  )
}
