import { createFileRoute } from '@tanstack/react-router'
import { useMessages, useSendMessage, useThread } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { driveApi } from '../../api/drive'
import { useState, useRef, useEffect, useCallback } from 'react'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { MessageItem } from '../../components/patterns/MessageItem'
import { ChatInput } from '../../components/patterns/ChatInput'
import { ChannelDrivePanel } from '../../components/patterns/ChannelDrivePanel'
import { Avatar, Heading, Spinner, Text } from '../../components/primitives'

export const Route = createFileRoute('/_workspace/channels/$channelId')({ component: ChannelChatView })

function ChannelChatView() {
  const { channelId } = Route.useParams()
  const { data, isLoading, error, refetch } = useMessages(channelId)
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const send = useSendMessage(channelId)
  const typingUsers = useWebSocketStore(s => s.typingUsers)
  const sendTyping = useWebSocketStore(s => s.sendTyping)
  const [input, setInput] = useState('')
  const [threadId, setThreadId] = useState<string | null>(null)
  const [showDrivePanel, setShowDrivePanel] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const msgs = data?.messages || []

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs.length])

  const handleSend = () => {
    const text = input.trim()
    if (!text) return
    setInput('')
    send.mutate(text)
  }

  /** Upload file to channel drive and send a message linking to it. */
  const handleFileUpload = useCallback(async (file: File) => {
    if (!wsId) return
    // 3-step upload to channel drive
    const driveData = await driveApi.channelDrive(channelId)
    const rootId = driveData?.items?.[0]?.parent_id || undefined
    const created = await driveApi.createFile(
      wsId, file.name, file.type || 'application/octet-stream', file.size, rootId,
    )
    await driveApi.uploadToStorage(created.upload_url, file)
    await driveApi.confirmFile(created.file_id)

    // Send message with linked entity so it renders as a file card
    send.mutate({
      content: `📎 ${file.name}`,
      linkedEntity: { type: 'drive_file', id: created.file_id },
    })
  }, [wsId, channelId, send])

  if (error) return <ErrorState title="Failed to load messages" message={error.message} onRetry={() => refetch()} />

  return (
    <div className="flex h-full -m-5">
      {/* Main chat column */}
      <div className={`flex-1 flex flex-col min-w-0 ${threadId || showDrivePanel ? 'border-r border-border' : ''}`}>
        {/* Channel header with files toggle */}
        <div className="flex items-center justify-end px-4 py-2 border-b border-border/50 bg-bg-tertiary/30">
          <button
            id="toggle-channel-drive"
            onClick={() => { setShowDrivePanel(v => !v); if (!showDrivePanel) setThreadId(null) }}
            className={`flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-[var(--radius-sm)]
              border-none cursor-pointer transition-all duration-150
              ${showDrivePanel
                ? 'bg-accent/15 text-accent font-medium'
                : 'bg-transparent text-text-muted hover:text-text-primary hover:bg-bg-hover'}`}
          >
            📁 Files
          </button>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto py-2">
          {isLoading ? <LoadingState /> :
           msgs.map((m, i) => (
            <MessageItem
              key={m.id || i}
              senderName={m.sender_name || ''}
              content={m.content}
              replyCount={m.reply_count}
              isGrouped={i > 0 && msgs[i-1]?.sender_id === m.sender_id}
              onReply={() => setThreadId(m.id)}
              linkedEntityType={m.linked_entity_type}
              linkedEntityId={m.linked_entity_id}
            />
          ))}
          <div ref={bottomRef} />
        </div>

        {/* Typing indicator */}
        {typingUsers[channelId] && (
          <div className="px-4 py-1.5 text-xs text-text-muted animate-fade-in">
            <span>{typingUsers[channelId]} is typing...</span>
          </div>
        )}

        {/* Chat input */}
        <ChatInput
          value={input}
          onChange={setInput}
          onSend={handleSend}
          onTyping={() => sendTyping(channelId)}
          onFileUpload={handleFileUpload}
          isPending={send.isPending}
          error={send.error?.message ?? null}
        />
      </div>

      {/* Side panels — only one visible at a time */}
      {threadId && <ThreadPanel messageId={threadId} onClose={() => setThreadId(null)} />}
      {showDrivePanel && !threadId && <ChannelDrivePanel channelId={channelId} onClose={() => setShowDrivePanel(false)} />}
    </div>
  )
}

function ThreadPanel({ messageId, onClose }: { messageId: string; onClose: () => void }) {
  const { data, isLoading, error, refetch } = useThread(messageId)
  const replies = data?.messages || []

  return (
    <div className="w-[340px] flex-shrink-0 flex flex-col bg-bg-secondary animate-slide-left">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <Heading as="h4">Thread</Heading>
        <button
          onClick={onClose}
          className="w-6 h-6 flex items-center justify-center rounded text-text-muted
            hover:text-text-primary hover:bg-bg-hover cursor-pointer border-none
            bg-transparent transition-colors"
        >
          ✕
        </button>
      </div>

      {/* Replies count */}
      <div className="px-4 py-2 border-b border-border/50">
        <Text variant="caption" muted>{replies.length} replies</Text>
      </div>

      {/* Thread messages */}
      <div className="flex-1 overflow-y-auto py-2">
        {error ? (
          <ErrorState title="Failed to load thread" message={error.message} onRetry={() => refetch()} />
        ) : isLoading ? (
          <div className="flex justify-center py-6"><Spinner /></div>
        ) : (
          replies.map((r, i) => (
            <div key={r.id || i} className="flex gap-2 px-4 py-1.5">
              <Avatar name={r.sender_name || '?'} size="sm" />
              <div className="flex-1 min-w-0">
                <p className="text-sm text-text-secondary m-0 break-words">{r.content}</p>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
