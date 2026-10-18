import { useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { ArrowLeft, Menu, X } from 'lucide-react'
import { useDeviceBack } from '../../hooks/useDeviceBack'
import { useChatSearchAddress } from '../../hooks/useChatSearchAddress'
import { useChatSuggestions } from '../../hooks/useChatSuggestions'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useSuggestionPanel } from '../../hooks/useSuggestionPanel'
import type { Conversation } from '../../lib/conversations'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'
import type { PeopleDirectory } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, IconButton, SearchField } from '../primitives'
import { SearchSuggestions } from '../patterns/SearchSuggestions'
import { ChatSearchResults } from './ChatSearchResults'

/**
 * Tìm trong chat on a phone (DESIGN.md "Gợi ý trong ô tìm", Điện thoại): a full-screen layer over the Chat list.
 * Its pill sits exactly where the shell's pill is, so the pill seems to stay put and turn into the search box: ☰
 * turns into ←, the avatar fades away, the accent ring fades in, the list behind fades and the suggestions rise.
 * Before anything is typed it lists Tìm gần đây (each with its ✕, and Xoá tất cả) and Trò chuyện gần đây; typed
 * words suggest people, groups and messages at once; Enter shows the results under the pill, with the filter
 * chips. The words and the filters are in the address. A modal layer: focus goes to the box and returns to what
 * opened it, Esc and ← close.
 */
export function ChatSearchScreen({ conversations, people, onClose }: {
  conversations: Conversation[]
  people: PeopleDirectory
  onClose: () => void
}) {
  const m = useMotionPresets()
  const user = useAuthStore((s) => s.user)
  const address = useChatSearchAddress()
  // Reached again by the device back (from a result's conversation): the device has drawn that step, so nothing plays in over it.
  const { byDevice } = useDeviceBack()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState(address.q)
  // Back and forward change the words in the address; the box follows them.
  const [seen, setSeen] = useState(address.q)
  if (address.q !== seen) {
    setSeen(address.q)
    setQuery(address.q)
  }
  // True from the first touch of the box until its words are submitted: only then are messages asked for as suggestions.
  const [editing, setEditing] = useState(!address.q)
  // On results (a reload, or back to them) the keyboard does not come up by itself; the box takes the focus when it is used.
  const [initialFocus] = useState(() => (address.q ? undefined : inputRef))
  useModalFocus({ active: true, surfaceRef, onClose, initialFocusRef: initialFocus })

  const submit = (words: string) => {
    const q = words.trim()
    if (!q) return
    suggestions.recent.add(q)
    address.commit(q, address.filters)
    panel.close()
    setEditing(false)
    // The keyboard goes away with the answer.
    inputRef.current?.blur()
  }
  const suggestions = useChatSuggestions({
    query, conversations, people, phone: true, enabled: editing,
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
    variant: 'screen',
    anchorRef: inputRef,
    initiallyOpen: !address.q,
    typing: true,
  })

  const fade = m.reduced ? DURATION.reducedFade : DURATION.quick
  // ☰ turns −90° and fades out while ← turns in from 90° (160ms); closing runs it backwards. No turning when motion is reduced.
  const spin = m.reduced ? 0 : 90
  const menuMotion = {
    initial: { opacity: 1, rotate: 0 },
    animate: { opacity: 0, rotate: -spin },
    exit: { opacity: 1, rotate: 0 },
    transition: { duration: fade, ease: EASE.out },
  }
  const backMotion = {
    initial: { opacity: 0, rotate: spin },
    animate: { opacity: 1, rotate: 0 },
    exit: { opacity: 0, rotate: spin },
    transition: { duration: fade, ease: EASE.out },
  }
  const myName = people.byUserId.get(user?.id ?? '')?.name ?? user?.username ?? ''

  return createPortal(
    <div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-label="Tìm trong chat"
      className="fixed inset-x-0 top-0 h-app z-modal flex flex-col min-h-0"
    >
      {/* The list under the layer fades out (120ms) as the layer fades in. */}
      <motion.div
        aria-hidden="true"
        initial={byDevice ? false : { opacity: 0 }}
        animate={{ opacity: 1, transition: { duration: DURATION.reducedFade, ease: EASE.in } }}
        exit={{ opacity: 0, transition: { duration: DURATION.reducedFade, ease: EASE.in } }}
        className="absolute inset-0 bg-base"
      />
      {/* Where the shell's own pill is: 12 from the sides, 12 under the safe area. */}
      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          submit(query)
        }}
        className="relative shrink-0 px-3 pb-1 pt-[calc(env(safe-area-inset-top,0px)+0.75rem)]"
      >
        <SearchField
          pill
          ref={inputRef}
          moduleSearch
          label="Tìm trong chat"
          value={query}
          {...panel.inputProps}
          onChange={(e) => {
            setQuery(e.target.value)
            setEditing(true)
          }}
          onFocus={() => {
            panel.inputProps.onFocus()
            setEditing(true)
          }}
          enterKeyHint="search"
          leading={
            <span className="relative shrink-0 w-11 h-11">
              <motion.span aria-hidden="true" {...menuMotion} initial={byDevice ? false : menuMotion.initial} className="absolute inset-0 grid place-items-center text-ink pointer-events-none">
                <Menu size={20} strokeWidth={1.75} />
              </motion.span>
              <motion.span {...backMotion} initial={byDevice ? false : backMotion.initial} className="absolute inset-0">
                <IconButton type="button" aria-label="Đóng tìm kiếm" title="Quay lại" shape="round" size="lg" className="w-11 h-11" onClick={onClose}>
                  <ArrowLeft size={20} strokeWidth={1.75} />
                </IconButton>
              </motion.span>
            </span>
          }
          trailing={query ? (
            <IconButton
              type="button"
              aria-label="Xoá"
              title="Xoá"
              shape="round"
              size="lg"
              className="w-11 h-11"
              onClick={() => {
                setQuery('')
                setEditing(true)
                inputRef.current?.focus()
              }}
            >
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          ) : (
            <motion.span
              aria-hidden="true"
              initial={byDevice ? false : { opacity: 1 }}
              animate={{ opacity: 0 }}
              exit={{ opacity: 1 }}
              transition={{ duration: fade, ease: EASE.out }}
              className="grid place-items-center w-11 h-11 pointer-events-none"
            >
              <Avatar name={myName} hueKey={user?.id} src={people.byUserId.get(user?.id ?? '')?.avatarUrl} size={32} />
            </motion.span>
          )}
        />
      </form>

      <div className="relative flex-1 min-h-0 flex flex-col pb-[env(safe-area-inset-bottom,0px)]">
        {panel.visible ? (
          <div className="flex-1 min-h-0 overflow-y-auto overscroll-y-contain">
            <SearchSuggestions
              panel={panel}
              emptyText="Tìm tin nhắn, người và nhóm."
              emptyHint="Sau khi tìm, lọc theo người gửi, cuộc trò chuyện hay ngày."
            />
          </div>
        ) : address.q ? (
          <ChatSearchResults
            variant="phone"
            q={address.q}
            filters={address.filters}
            onFilters={(next) => address.commit(address.q, next)}
            conversations={conversations}
            people={people}
          />
        ) : null}
      </div>
    </div>,
    document.body,
  )
}
