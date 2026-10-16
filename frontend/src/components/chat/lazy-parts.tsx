import { lazy, Suspense, type ComponentProps } from 'react'

/**
 * The heavy parts of a conversation, each loaded on first use: the rich-text
 * composer and message editor (tiptap + prosemirror), the emoji picker (emoji-mart
 * and its data) and the markdown renderer. None of them is needed to paint a
 * list of messages, and the sign-in screen needs none. Every fallback is a
 * Skeleton of the part's own footprint, so nothing moves when it arrives.
 */

const loadChatEditor = () => import('./ChatEditor')
const loadEmojiPicker = () => import('./EmojiPicker')
const loadMessageEditBox = () => import('../spaces/MessageEditBox')
const loadMarkdownBody = () => import('./MarkdownBody')

const ChatEditorImpl = lazy(() => loadChatEditor().then((m) => ({ default: m.ChatEditor })))
const EmojiPickerImpl = lazy(() => loadEmojiPicker().then((m) => ({ default: m.EmojiPicker })))
const MessageEditBoxImpl = lazy(() => loadMessageEditBox().then((m) => ({ default: m.MessageEditBox })))
const MarkdownBodyImpl = lazy(() => loadMarkdownBody().then((m) => ({ default: m.MarkdownBody })))

/** Warm every lazy part so the first composer, picker or edit opens without a wait. */
export function prefetchChatParts(): Promise<unknown> {
  // A failed warm-up is harmless: the part loads, or reports, when it is first used.
  return Promise.all(
    [loadChatEditor, loadEmojiPicker, loadMessageEditBox, loadMarkdownBody].map((load) => load().catch(() => undefined)),
  )
}

/** The composer's footprint: its margin, its box, one line of text and the action row. */
function ComposerSkeleton({ tone }: { tone?: 'raised' | 'base' }) {
  const raised = tone !== 'base'
  return (
    <div className={`relative ${raised ? 'mx-4 mb-4' : 'mx-3 mb-3'}`} aria-busy="true">
      <div className={`grid gap-2 py-2.5 pr-2.5 pl-3.5 rounded-lg ${raised ? 'bg-raised' : 'bg-base'}`}>
        <div className="py-1">
          <div className="skeleton h-6 w-2/5 rounded-sm" />
        </div>
        <div className="skeleton h-9 max-md:h-11 w-full rounded-sm" />
      </div>
    </div>
  )
}

export function LazyChatEditor(props: ComponentProps<typeof ChatEditorImpl>) {
  return (
    <Suspense fallback={<ComposerSkeleton tone={props.tone} />}>
      <ChatEditorImpl {...props} />
    </Suspense>
  )
}

export function LazyEmojiPicker(props: ComponentProps<typeof EmojiPickerImpl>) {
  return (
    <Suspense fallback={<div aria-busy="true" className="skeleton w-88 h-108 rounded-overlay" />}>
      <EmojiPickerImpl {...props} />
    </Suspense>
  )
}

export function LazyMessageEditBox(props: ComponentProps<typeof MessageEditBoxImpl>) {
  return (
    <Suspense fallback={<div aria-busy="true" className="skeleton h-12 w-full rounded-lg" />}>
      <MessageEditBoxImpl {...props} />
    </Suspense>
  )
}

/** Markdown messages are the rare format; until the renderer arrives the text itself holds the place. */
export function LazyMarkdownBody({ content }: { content: string }) {
  return (
    <Suspense fallback={<p className="m-0 whitespace-pre-wrap" aria-busy="true">{content}</p>}>
      <MarkdownBodyImpl content={content} />
    </Suspense>
  )
}
