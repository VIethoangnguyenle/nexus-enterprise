import { createFileRoute } from '@tanstack/react-router'
import { useMessages, useSendMessage, useThread } from '../../hooks/useMessaging'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useState, useRef, useEffect } from 'react'

export const Route = createFileRoute('/_workspace/channels/$channelId')({ component: ChatView })

function ChatView() {
  const { channelId } = Route.useParams()
  const { data, isLoading } = useMessages(channelId)
  const send = useSendMessage(channelId)
  const typingUsers = useWebSocketStore(s => s.typingUsers)
  const sendTyping = useWebSocketStore(s => s.sendTyping)
  const [input, setInput] = useState('')
  const [threadId, setThreadId] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const msgs = data?.messages || []

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs.length])

  const handleSend = () => {
    const text = input.trim()
    if (!text) return
    setInput('')
    send.mutate(text)
  }

  const initials = (n: string) => n ? n.slice(0, 2).toUpperCase() : '?'

  return (
    <div className="chat-view-wrapper">
      <div className={`chat-main ${threadId ? 'with-thread' : ''}`}>
        <div className="chat-messages" style={{ flex: 1, overflow: 'auto', padding: '1rem' }}>
          {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
           msgs.map((m, i) => (
            <div key={m.id || i} className="message" style={{ marginTop: i > 0 && msgs[i-1]?.sender_id !== m.sender_id ? '0.75rem' : 0 }}>
              <div className="message-avatar">{initials(m.sender_name || '')}</div>
              <div className="message-body">
                <div className="message-header"><span className="message-sender">{m.sender_name}</span></div>
                <div className="message-content">{m.content}</div>
                <div className="message-actions-bar">
                  <button className="message-reply-btn" onClick={() => setThreadId(m.id)}>
                    💬 {m.reply_count ? `${m.reply_count} replies` : 'Reply'}
                  </button></div></div></div>
          ))}
          <div ref={bottomRef} />
        </div>
        {typingUsers[channelId] && <div className="chat-typing"><span>{typingUsers[channelId]} is typing...</span></div>}
        <div className="chat-input-area"><div className="chat-input-wrapper">
          <textarea className="chat-input" value={input} onChange={e => { setInput(e.target.value); sendTyping(channelId) }}
            onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend() } }} rows={1} placeholder="Message..." />
          <button className="chat-send-btn" onClick={handleSend} disabled={!input.trim()}>➤</button>
        </div></div>
      </div>
      {threadId && <ThreadPanel messageId={threadId} onClose={() => setThreadId(null)} />}
    </div>
  )
}

function ThreadPanel({ messageId, onClose }: { messageId: string; onClose: () => void }) {
  const { data, isLoading } = useThread(messageId)
  const replies = data?.messages || []
  return (
    <div className="thread-panel">
      <div className="thread-header"><h3 className="thread-title">Thread</h3><button className="thread-close" onClick={onClose}>✕</button></div>
      <div className="thread-divider"><span>{replies.length} replies</span></div>
      <div className="thread-messages">
        {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
         replies.map((r, i) => (
          <div key={r.id || i} className="message thread-reply">
            <div className="message-avatar" style={{width:28,height:28,fontSize:'0.65rem'}}>{(r.sender_name||'?').slice(0,2).toUpperCase()}</div>
            <div className="message-body"><div className="message-content">{r.content}</div></div></div>
        ))}
      </div>
    </div>
  )
}
