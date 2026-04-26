import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMessagingStore, useAuthStore } from '../store'

export default function ChatView() {
  const { channelId } = useParams()
  const user = useAuthStore(s => s.user)
  const {
    activeChannel, messages, hasMore, loadingMessages,
    selectChannel, fetchMessages, sendMessage, sendTyping, typingUsers
  } = useMessagingStore()
  const channels = useMessagingStore(s => s.channels)
  const dms = useMessagingStore(s => s.dms)

  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef(null)
  const messagesRef = useRef(null)
  const typingTimer = useRef(null)

  // Auto-select channel from URL
  useEffect(() => {
    if (!channelId) return
    const all = [...(channels || []), ...(dms || [])]
    const ch = all.find(c => c.id === channelId)
    if (ch && ch.id !== activeChannel?.id) {
      selectChannel(ch)
    }
  }, [channelId, channels, dms])

  // Scroll to bottom on new messages
  useEffect(() => {
    if (bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages.length])

  const handleSend = async () => {
    const text = input.trim()
    if (!text || !channelId) return
    setSending(true)
    setInput('')
    try {
      await sendMessage(channelId, text)
    } catch (err) {
      console.error(err)
      setInput(text) // restore on error
    }
    setSending(false)
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleInput = (e) => {
    setInput(e.target.value)
    // Typing indicator (debounced)
    if (typingTimer.current) clearTimeout(typingTimer.current)
    typingTimer.current = setTimeout(() => {
      sendTyping(channelId)
    }, 300)
  }

  const handleLoadMore = () => {
    if (messages.length > 0 && hasMore) {
      const oldest = messages[0]
      if (oldest?.created_at || oldest?.createdAt) {
        fetchMessages(channelId, oldest.created_at || oldest.createdAt)
      }
    }
  }

  const initials = (name) => name ? name.slice(0, 2).toUpperCase() : '?'
  const formatTime = (ts) => {
    if (!ts) return ''
    try {
      const d = new Date(ts.seconds ? ts.seconds * 1000 : ts)
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } catch {
      return ''
    }
  }

  const channelName = activeChannel?.name || 'Channel'
  const isChannel = activeChannel?.channel_type !== 'dm' && activeChannel?.channelType !== 'dm'
  const typingText = typingUsers[channelId]

  return (
    <>
      <div className="content-topbar">
        <div className="topbar-title">
          <span className="hash">{isChannel ? '#' : '●'}</span>
          {channelName}
        </div>
        <div className="topbar-actions">
          <button className="btn btn-secondary btn-sm">👥 Members</button>
        </div>
      </div>

      <div className="chat-container" style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="chat-messages" ref={messagesRef}>
          {hasMore && (
            <div className="chat-load-more">
              <button className="btn btn-secondary btn-sm" onClick={handleLoadMore} disabled={loadingMessages}>
                {loadingMessages ? <span className="spinner" /> : 'Load older messages'}
              </button>
            </div>
          )}

          {!loadingMessages && messages.length === 0 && (
            <div className="empty-state" style={{ padding: '3rem 1rem' }}>
              <div className="icon">💬</div>
              <h3>No messages yet</h3>
              <p>Be the first to say something in {isChannel ? `#${channelName}` : channelName}</p>
            </div>
          )}

          {messages.map((msg, i) => {
            const senderName = msg.sender_name || msg.senderName || 'Unknown'
            const showAvatar = i === 0 || (messages[i - 1]?.sender_id || messages[i - 1]?.senderId) !== (msg.sender_id || msg.senderId)

            return (
              <div key={msg.id || i} className="message" style={{ marginTop: showAvatar ? '0.75rem' : 0 }}>
                {showAvatar ? (
                  <div className="message-avatar">{initials(senderName)}</div>
                ) : (
                  <div style={{ width: 36, flexShrink: 0 }} />
                )}
                <div className="message-body">
                  {showAvatar && (
                    <div className="message-header">
                      <span className="message-sender">{senderName}</span>
                      <span className="message-time">{formatTime(msg.created_at || msg.createdAt)}</span>
                    </div>
                  )}
                  <div className="message-content">{msg.content}</div>
                </div>
              </div>
            )
          })}
          <div ref={bottomRef} />
        </div>

        <div className="chat-typing">
          {typingText && <span>{typingText} is typing...</span>}
        </div>

        <div className="chat-input-area">
          <div className="chat-input-wrapper">
            <textarea
              className="chat-input"
              placeholder={`Message ${isChannel ? '#' + channelName : channelName}`}
              value={input}
              onChange={handleInput}
              onKeyDown={handleKeyDown}
              rows={1}
            />
            <button className="chat-send-btn" onClick={handleSend} disabled={!input.trim() || sending}>
              ➤
            </button>
          </div>
        </div>
      </div>
    </>
  )
}
