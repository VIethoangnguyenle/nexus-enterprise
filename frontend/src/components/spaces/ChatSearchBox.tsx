import { useEffect, useRef, useState } from 'react'
import { useNavigate, useRouterState, useSearch } from '@tanstack/react-router'
import { X } from 'lucide-react'
import { useChatSuggestions } from '../../hooks/useChatSuggestions'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { useSuggestionPanel } from '../../hooks/useSuggestionPanel'
import { filtersFromUrl, parseChatSearch, urlFromFilters } from '../../lib/chat-search'
import type { Conversation } from '../../lib/conversations'
import type { PeopleDirectory } from '../../lib/people'
import { isTypingTarget } from '../../lib/typing-target'
import { IconButton } from '../primitives'
import { SearchBox } from '../patterns/SearchBox'
import { Kbd } from '../patterns/SearchSuggestions'

/** The route of the results page; its address carries the words and the filters. */
export const CHAT_SEARCH_PATH = '/channels/search'

/**
 * Tìm trong chat on a desktop (DESIGN.md "Gợi ý trong ô tìm"): the pill at the head of the conversation list. It
 * opens into a surface with Tìm gần đây and Trò chuyện gần đây, and once words are typed with people, groups and
 * messages that match; Enter (or "Xem tất cả kết quả") opens the results page. `/` anywhere in Chat focuses it
 * and Esc steps back: close the suggestions, clear the words, leave the box for where the focus was.
 */
export function ChatSearchBox({ conversations, people }: { conversations: Conversation[]; people: PeopleDirectory }) {
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  /** Set when `/` took the focus: the last Esc then leaves the box, for `returnTo` (where the focus was) or, with none, for the page. */
  const viaSlash = useRef(false)
  const returnTo = useRef<HTMLElement | null>(null)
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const address = parseChatSearch(useSearch({ strict: false }) as Record<string, unknown>)
  const onResults = pathname === CHAT_SEARCH_PATH
  const addressWords = onResults ? address.q ?? '' : ''

  const [query, setQuery] = useState(addressWords)
  // Back, forward and a link change the words in the address; the box follows them.
  const [seen, setSeen] = useState(addressWords)
  if (addressWords !== seen) {
    setSeen(addressWords)
    setQuery(addressWords)
  }
  const [focused, setFocused] = useState(false)
  // Below 1024px this column is not on screen (the phone has its own search layer), so its key is not taken either.
  const hidden = useBottomBarLayout()

  const submit = (words: string) => {
    const q = words.trim()
    if (!q) return
    suggestions.recent.add(q)
    panel.close()
    // Refining the words on the results page keeps the filters; coming from anywhere else starts without.
    const keep = onResults ? filtersFromUrl(address) : {}
    void navigate({ to: CHAT_SEARCH_PATH, search: urlFromFilters(q, keep), replace: onResults })
  }

  const suggestions = useChatSuggestions({
    query, conversations, people, phone: false, enabled: focused,
    onReuse: (q) => {
      setQuery(q)
      inputRef.current?.focus()
    },
    onSubmit: submit,
  })
  const panel = useSuggestionPanel({
    value: query,
    groups: suggestions.groups,
    note: suggestions.note,
    variant: 'popover',
    anchorRef: inputRef,
    typing: true,
    showWhenEmpty: true,
  })

  // `/` anywhere in Chat, except while typing somewhere or with a dialog open, jumps to the box.
  useEffect(() => {
    if (hidden) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '/' || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target)) return
      if (document.querySelector('[aria-modal="true"]')) return
      e.preventDefault()
      const from = document.activeElement
      viaSlash.current = true
      returnTo.current = from instanceof HTMLElement && from !== document.body ? from : null
      inputRef.current?.focus()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [hidden])

  const clear = () => {
    setQuery('')
    inputRef.current?.focus()
  }

  return (
    <SearchBox
      ref={inputRef}
      panel={panel}
      label="Tìm trong chat"
      moduleSearch
      tone="sunk"
      shape="pill"
      tall
      wide
      slotClassName="w-full"
      emptyText="Tìm tin nhắn, người và nhóm."
      listLabel="Gợi ý tìm kiếm"
      value={query}
      {...panel.inputProps}
      onChange={(e) => setQuery(e.target.value)}
      onFocus={() => {
        panel.inputProps.onFocus()
        setFocused(true)
      }}
      onBlur={(e) => {
        panel.inputProps.onBlur(e)
        setFocused(false)
      }}
      onPointerDown={() => {
        viaSlash.current = false
        returnTo.current = null
      }}
      onKeyDown={(e) => {
        panel.inputProps.onKeyDown(e)
        if (e.defaultPrevented) return
        if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
          e.preventDefault()
          submit(query)
        } else if (e.key === 'Escape') {
          e.preventDefault()
          e.stopPropagation()
          if (query) {
            panel.holdClosed()
            setQuery('')
          } else if (viaSlash.current) {
            // The last Esc leaves the box for where the focus was before `/` (the page itself when it was nowhere);
            // a box opened with the pointer keeps the focus.
            const back = returnTo.current
            viaSlash.current = false
            returnTo.current = null
            inputRef.current?.blur()
            back?.focus()
          }
        }
      }}
      trailing={query ? (
        <IconButton aria-label="Xoá chữ đã nhập" title="Xoá chữ đã nhập" shape="round" onClick={clear}>
          <X size={16} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
      ) : (
        <span aria-hidden="true" className={`mr-2 transition-opacity duration-quick ease-out ${panel.visible ? 'opacity-0' : ''}`}>
          <Kbd>/</Kbd>
        </span>
      )}
    />
  )
}
