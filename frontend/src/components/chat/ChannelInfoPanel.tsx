import { useState, useMemo } from 'react'
import { Heading } from '../primitives'
import { useChannelMembers, useAddChannelMember, useRemoveChannelMember, usePins, useSearch, useUpdateChannel } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useChannelDrive } from '../../hooks/useDrive'
import { useContacts } from '../../hooks/useContacts'
import { FilePreviewCard } from '../patterns/FilePreviewCard'
import { Modal } from '../composites/Modal'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { Button, IconButton, Spinner } from '../primitives'
import { Users, Pin, Search, FolderOpen, Settings, X, UserPlus, UserMinus } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

/** Extracts plain text from HTML content for previews. */
function stripHtml(html: string): string {
  if (!html) return ''
  if (typeof DOMParser !== 'undefined') {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    return doc.body.textContent || ''
  }
  return html.replace(/<[^>]*>/g, '')
}

interface ChannelInfoPanelProps {
  channelId: string
  channelName: string
  wsId: string
  onClose: () => void
  initialTab?: string
}

type TabId = 'members' | 'pins' | 'search' | 'files' | 'settings'

/** Lark-style right panel with tabbed content: Members, Pins, Search, Files, Settings. */
export function ChannelInfoPanel({ channelId, channelName, wsId, onClose, initialTab = 'members' }: ChannelInfoPanelProps) {
  const [activeTab, setActiveTab] = useState<TabId>(initialTab as TabId)

  const tabs: { id: TabId; label: string; icon: LucideIcon }[] = [
    { id: 'members', label: 'Members', icon: Users },
    { id: 'pins', label: 'Pins', icon: Pin },
    { id: 'search', label: 'Search', icon: Search },
    { id: 'files', label: 'Files', icon: FolderOpen },
    { id: 'settings', label: 'Settings', icon: Settings },
  ]

  return (
    <div className="fixed inset-0 z-40 lg:relative lg:inset-auto lg:z-auto lg:w-[340px]
      flex-shrink-0 flex flex-col bg-surface-container-lowest animate-panel-slide lg:border-l lg:border-outline-variant">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-outline-variant">
        <h3 className="text-sm font-semibold text-on-surface">{channelName}</h3>
        <IconButton
          onClick={onClose}
          aria-label="Close"
          size="sm"
        >
          <X size={16} />
        </IconButton>
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-outline-variant px-2">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1 px-3 py-2 text-xs border-none cursor-pointer
              transition-all duration-150 bg-transparent
              ${activeTab === tab.id
                ? 'text-primary font-medium border-b-2 border-primary -mb-px'
                : 'text-on-surface-variant hover:text-on-surface'}`}
          >
            <tab.icon size={14} />
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto">
        {activeTab === 'members' && <MembersTab channelId={channelId} channelName={channelName} wsId={wsId} />}
        {activeTab === 'pins' && <PinsTab channelId={channelId} />}
        {activeTab === 'search' && <SearchTab channelId={channelId} />}
        {activeTab === 'files' && <FilesTab channelId={channelId} wsId={wsId} />}
        {activeTab === 'settings' && <SettingsTab channelId={channelId} channelName={channelName} />}
      </div>
    </div>
  )
}

/** Members tab — list of channel members with add/remove actions. */
function MembersTab({ channelId, channelName, wsId }: { channelId: string; channelName: string; wsId: string }) {
  const { data, isLoading } = useChannelMembers(channelId)
  const members = data?.members || []
  const [showAddModal, setShowAddModal] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<{ nodeId: string; username: string } | null>(null)
  const removeMutation = useRemoveChannelMember(channelId)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)

  return (
    <div className="p-3">
      {/* Header with count and Add button */}
      <div className="flex items-center justify-between mb-3">
        <div className="text-xs text-on-surface-variant">{members.length} members</div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowAddModal(true)}
        >
          <UserPlus size={14} />
          Add
        </Button>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-4"><Spinner size="sm" /></div>
      ) : (() => {
        const online = members.filter((m) => !!onlineUsers[m.user_id])
        const offline = members.filter((m) => !onlineUsers[m.user_id])
        return (
          <div className="space-y-1">
            {/* ONLINE section */}
            {online.length > 0 && (
              <>
                <div className="px-2 pt-1 pb-0.5 font-label-caps text-label-caps text-green-600">
                  Online — {online.length}
                </div>
                {online.map((m) => (
                  <MemberRow key={m.ngac_node_id || m.user_id} member={m} isOnline onRemove={() => setRemoveTarget({ nodeId: m.ngac_node_id, username: m.username })} />
                ))}
              </>
            )}
            {/* OFFLINE section */}
            {offline.length > 0 && (
              <>
                <div className="px-2 pt-2 pb-0.5 font-label-caps text-label-caps text-on-surface-variant">
                  Offline — {offline.length}
                </div>
                {offline.map((m) => (
                  <MemberRow key={m.ngac_node_id || m.user_id} member={m} isOnline={false} onRemove={() => setRemoveTarget({ nodeId: m.ngac_node_id, username: m.username })} />
                ))}
              </>
            )}
          </div>
        )
      })()}

      {/* Add Member Modal */}
      {showAddModal && (
        <AddMemberModal
          channelId={channelId}
          channelName={channelName}
          wsId={wsId}
          existingMembers={members}
          onClose={() => setShowAddModal(false)}
        />
      )}

      {/* Remove Confirmation Dialog */}
      <ConfirmDialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        onConfirm={() => {
          if (removeTarget) {
            removeMutation.mutate(
              { nodeId: removeTarget.nodeId },
              { onSuccess: () => setRemoveTarget(null) },
            )
          }
        }}
        title="Remove Member"
        description={
          <>Remove <strong>{removeTarget?.username}</strong> from <strong>#{channelName}</strong>?</>
        }
        warning="This member will lose access to the channel."
        icon={<UserMinus size={22} className="text-error" />}
        iconBg="bg-error-container"
        confirmLabel="Remove"
        confirmVariant="error"
        loading={removeMutation.isPending}
      />
    </div>
  )
}
/** Single member row with online dot and hover remove button. */
function MemberRow({ member: m, isOnline, onRemove }: {
  member: { user_id: string; username: string; ngac_node_id: string }
  isOnline: boolean
  onRemove: () => void
}) {
  return (
    <div className="group flex items-center gap-2 px-2 py-2 rounded hover:bg-surface-container-high transition-colors">
      <div className="relative flex-shrink-0">
        <div className="w-7 h-7 rounded-full bg-primary-container flex items-center justify-center text-xs text-on-primary-container font-medium">
          {m.username?.[0]?.toUpperCase() || '?'}
        </div>
        <span className={`absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-surface-container-lowest ${isOnline ? 'bg-green-500' : 'bg-slate-300'}`} />
      </div>
      <span className="text-sm text-on-surface flex-1 truncate">{m.username}</span>
      <IconButton
        onClick={onRemove}
        className="opacity-0 group-hover:opacity-100 text-on-surface-variant hover:text-error"
        aria-label={`Remove ${m.username}`}
        size="sm"
      >
        <X size={14} />
      </IconButton>
    </div>
  )
}

/** Modal for adding workspace members to a channel. */
function AddMemberModal({
  channelId,
  channelName,
  wsId,
  existingMembers,
  onClose,
}: {
  channelId: string
  channelName: string
  wsId: string
  existingMembers: { user_id: string; username: string; ngac_node_id: string }[]
  onClose: () => void
}) {
  const [search, setSearch] = useState('')
  const { data: contactsData, isLoading: loadingContacts } = useContacts(wsId)
  const addMutation = useAddChannelMember(channelId)

  const existingNodeIds = useMemo(
    () => new Set(existingMembers.map((m) => m.ngac_node_id)),
    [existingMembers],
  )

  const filtered = useMemo(() => {
    const contacts = contactsData?.contacts || []
    if (!search.trim()) return contacts
    const q = search.toLowerCase()
    return contacts.filter(
      (c) =>
        c.username.toLowerCase().includes(q) ||
        c.display_name.toLowerCase().includes(q) ||
        c.email.toLowerCase().includes(q),
    )
  }, [contactsData, search])

  return (
    <Modal onClose={onClose} size="sm">
      <Modal.Header onClose={onClose}>Add Member to #{channelName}</Modal.Header>
      <div className="p-4">
        {/* Search input */}
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search workspace members..."
          autoFocus
          className="w-full bg-surface-container border border-outline-variant rounded px-3 py-2
            text-sm text-on-surface placeholder:text-on-surface-variant outline-none
            focus:border-primary focus:ring-2 focus:ring-primary/10 transition-all mb-3"
        />

        {/* Results */}
        <div className="max-h-[300px] overflow-y-auto space-y-1">
          {loadingContacts ? (
            <div className="flex justify-center py-4"><Spinner size="sm" /></div>
          ) : filtered.length === 0 ? (
            <div className="text-xs text-on-surface-variant text-center py-4">
              {search ? 'No matching members' : 'No workspace members found'}
            </div>
          ) : (
            filtered.map((c) => {
              const alreadyAdded = existingNodeIds.has(c.ngac_node_id)
              return (
                <div
                  key={c.ngac_node_id || c.user_id}
                  className="flex items-center gap-2 px-2 py-2 rounded hover:bg-surface-container-high transition-colors"
                >
                  <div className="w-7 h-7 rounded-full bg-primary-container flex items-center justify-center text-xs text-on-primary-container font-medium flex-shrink-0">
                    {(c.display_name || c.username)?.[0]?.toUpperCase() || '?'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-on-surface truncate">{c.display_name || c.username}</div>
                    {c.email && <div className="text-[10px] text-on-surface-variant truncate">{c.email}</div>}
                  </div>
                  {alreadyAdded ? (
                    <span className="text-[10px] text-on-surface-variant px-2 py-0.5 rounded bg-surface-container">
                      Added
                    </span>
                  ) : (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        addMutation.mutate(
                          { ngacNodeId: c.ngac_node_id, username: c.display_name || c.username },
                          { onSuccess: () => {} },
                        )
                      }
                      disabled={addMutation.isPending}
                    >
                      Add
                    </Button>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>
    </Modal>
  )
}

/** Pins tab — list of pinned messages. */
function PinsTab({ channelId }: { channelId: string }) {
  const { data, isLoading } = usePins(channelId)
  const pins = data?.pins || []

  return (
    <div className="p-3">
      {isLoading ? (
        <div className="text-xs text-on-surface-variant text-center py-4">Loading...</div>
      ) : pins.length === 0 ? (
        <div className="text-xs text-on-surface-variant text-center py-8">No pinned messages</div>
      ) : (
        <div className="space-y-2">
          {pins.map((p) => (
              <div key={p.message?.id} className="bg-surface-container rounded p-3 border border-outline-variant">
              <div className="flex items-center gap-2 mb-1">
                <span className="text-xs font-medium text-on-surface">{p.message?.sender_name}</span>
                <span className="text-[10px] text-on-surface-variant">pinned by {p.pinned_by}</span>
              </div>
              <p className="text-sm text-on-surface-variant m-0 line-clamp-3">{stripHtml(p.message?.content || '')}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/** Search tab — search messages in this channel. */
function SearchTab({ channelId }: { channelId: string }) {
  const [query, setQuery] = useState('')
  const { data, isLoading } = useSearch(channelId, query)
  const results = data?.messages || []

  return (
    <div className="p-3">
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search messages..."
        className="w-full bg-surface-container border border-outline-variant rounded px-3 py-2
          text-sm text-on-surface placeholder:text-on-surface-variant outline-none
          focus:border-primary focus:ring-2 focus:ring-primary/10 transition-all mb-3"
      />
      {isLoading ? (
        <div className="text-xs text-on-surface-variant text-center py-4">Searching...</div>
      ) : query.length < 2 ? (
        <div className="text-xs text-on-surface-variant text-center py-4">Type at least 2 characters</div>
      ) : results.length === 0 ? (
        <div className="text-xs text-on-surface-variant text-center py-4">No results</div>
      ) : (
        <div className="space-y-2">
          {results.map((m) => (
            <div key={m.id} className="bg-surface-container rounded p-3 border border-outline-variant">
              <div className="text-xs font-medium text-on-surface mb-1">{m.sender_name}</div>
              <p className="text-sm text-on-surface-variant m-0 line-clamp-2">{stripHtml(m.content)}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/** Files tab — shows files uploaded to this channel's drive. */
function FilesTab({ channelId, wsId }: { channelId: string; wsId: string }) {
  const { data, isLoading } = useChannelDrive(wsId, channelId)
  const items = data?.items || []
  const files = items.filter(i => i.item_type === 'file' && i.status === 'active')

  return (
    <div className="p-3">
      {isLoading ? (
        <div className="text-xs text-on-surface-variant text-center py-4">Loading files...</div>
      ) : files.length === 0 ? (
        <div className="text-xs text-on-surface-variant text-center py-8">
          No files shared yet.
          <br />
          Use the attach button in the editor to share files.
        </div>
      ) : (
        <div className="space-y-1">
          <div className="text-xs text-on-surface-variant mb-2">{files.length} {files.length === 1 ? 'file' : 'files'}</div>
          {files.map(file => (
            <FilePreviewCard key={file.id} item={file} />
          ))}
        </div>
      )}
    </div>
  )
}

/** Settings tab — channel info and rename. */
function SettingsTab({ channelId, channelName }: { channelId: string; channelName: string }) {
  const [name, setName] = useState(channelName)
  const [editing, setEditing] = useState(false)
  const updateMutation = useUpdateChannel(channelId)

  const handleSave = () => {
    if (name.trim() && name !== channelName) {
      updateMutation.mutate({ name: name.trim() }, {
        onSuccess: () => setEditing(false),
      })
    } else {
      setEditing(false)
    }
  }

  return (
    <div className="p-4 space-y-4">
      <div>
        <label className="text-xs text-on-surface-variant mb-1 block">Channel Name</label>
        {editing ? (
          <div className="flex items-center gap-2">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
              onKeyDown={(e) => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') { setName(channelName); setEditing(false) } }}
              className="flex-1 bg-surface-container border border-outline-variant rounded px-3 py-1.5
                text-sm text-on-surface placeholder:text-on-surface-variant outline-none
                focus:border-primary focus:ring-2 focus:ring-primary/10 transition-all"
            />
            <Button variant="primary" size="sm" onClick={handleSave} disabled={updateMutation.isPending}>
              Save
            </Button>
          </div>
        ) : (
          <button
            onClick={() => setEditing(true)}
            className="text-sm text-on-surface font-medium cursor-pointer bg-transparent border-none
              hover:text-primary transition-colors p-0"
          >
            # {channelName}
          </button>
        )}
      </div>
      <div>
        <label className="text-xs text-on-surface-variant mb-1 block">Notifications</label>
        <div className="text-sm text-on-surface-variant">All messages</div>
      </div>
    </div>
  )
}
