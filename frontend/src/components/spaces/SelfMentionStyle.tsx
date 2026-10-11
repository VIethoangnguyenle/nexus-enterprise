import { useEffect } from 'react'

const USER_ID = /^[\w-]{1,64}$/

/** The rule that washes the reader's own @mention; every other @name stays accent text (index.css). */
export function selfMentionRule(userId: string | undefined): string | null {
  if (!userId || !USER_ID.test(userId)) return null
  return `.message-html [data-mention="${userId}"]{background:var(--color-accent-wash);padding:0 4px;border-radius:4px}`
}

/**
 * Only a mention of the reader is washed in the stream and in the thread panel
 * (design/mockups/shortcuts.html §1). A message carries `data-mention=<user id>`
 * and CSS cannot compare it to the signed-in user, so the rule is written here.
 */
export function SelfMentionStyle({ userId }: { userId: string | undefined }) {
  const rule = selfMentionRule(userId)
  // In the head, not the page: the rule names the reader's id, which must not sit in the screen's text.
  useEffect(() => {
    if (!rule) return
    const el = document.createElement('style')
    el.setAttribute('data-self-mention', '')
    el.textContent = rule
    document.head.appendChild(el)
    return () => el.remove()
  }, [rule])
  return null
}
