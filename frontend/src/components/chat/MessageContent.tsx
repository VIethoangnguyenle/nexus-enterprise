import { sanitizeHtml } from '../../lib/sanitize-html'
import { LazyMarkdownBody } from './lazy-parts'

interface MessageContentProps {
  content: string
  contentFormat?: string
}

/**
 * Renders message content as markdown or HTML. HTML is the composer's rich text, filtered again here;
 * a mention in it is a `span[data-mention]` node, styled by `.message-html`, never found by pattern.
 */
export function MessageContent({ content, contentFormat }: MessageContentProps) {
  // If content is HTML from TipTap, render it directly
  if (contentFormat === 'html' || content.startsWith('<')) {
    return (
      <div
        className="message-html text-16 md:text-md text-ink break-words"
        dangerouslySetInnerHTML={{ __html: sanitizeHtml(content) }}
      />
    )
  }

  // Markdown rendering; the renderer is fetched the first time a markdown message is shown.
  return (
    <div className="message-markdown text-16 md:text-md text-ink break-words [&_p]:m-0 [&_blockquote]:border-l-2 [&_blockquote]:border-primary/30 [&_blockquote]:pl-3 [&_blockquote]:my-1 [&_blockquote]:text-on-surface-variant [&_ul]:pl-5 [&_ol]:pl-5 [&_li]:my-1 [&_a]:text-primary [&_a]:underline [&_strong]:text-on-surface">
      <LazyMarkdownBody content={content} />
    </div>
  )
}
