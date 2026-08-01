import { useCallback, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Copy, Check } from 'lucide-react'
import { Button } from '../primitives'

interface MessageContentProps {
  content: string
  contentFormat?: string
}

/** Renders message content as markdown or HTML. Supports code blocks with copy, @mention highlighting. */
export function MessageContent({ content, contentFormat }: MessageContentProps) {
  // If content is HTML from TipTap, render it directly
  if (contentFormat === 'html' || content.startsWith('<')) {
    return (
      <div
        className="message-html text-body text-on-surface-variant leading-relaxed break-words"
        dangerouslySetInnerHTML={{ __html: highlightMentions(content) }}
      />
    )
  }

  // Markdown rendering with custom code block component
  return (
    <div className="message-markdown text-body text-on-surface-variant leading-relaxed break-words [&_p]:m-0 [&_blockquote]:border-l-2 [&_blockquote]:border-primary/30 [&_blockquote]:pl-3 [&_blockquote]:my-1 [&_blockquote]:text-on-surface-variant [&_ul]:pl-5 [&_ol]:pl-5 [&_li]:my-1 [&_a]:text-primary [&_a]:underline [&_strong]:text-on-surface">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code: CodeBlock,
        }}
      >
        {highlightMentionsMarkdown(content)}
      </ReactMarkdown>
    </div>
  )
}

/** Custom code renderer — inline code or fenced code block with dark bg + copy button. */
function CodeBlock({ className, children, ...props }: any) {
  const match = /language-(\w+)/.exec(className || '')
  const isBlock = !!match || (typeof children === 'string' && children.includes('\n'))
  const codeStr = String(children).replace(/\n$/, '')

  if (!isBlock) {
    // Inline code
    return (
      <code className="bg-surface-container px-1.5 py-0.5 rounded text-small text-primary font-mono" {...props}>
        {children}
      </code>
    )
  }

  // Fenced code block
  return <FencedCodeBlock code={codeStr} language={match?.[1]} />
}

function FencedCodeBlock({ code, language }: { code: string; language?: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(code).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }, [code])

  return (
    <div className="my-2 rounded-lg overflow-hidden border border-outline-variant/30">
      {/* Header bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-surface-container-high">
        {language && (
          <span className="text-micro font-mono text-on-surface-variant">{language}</span>
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={handleCopy}
          className="ml-auto"
          title="Copy code"
        >
          {copied ? (
            <><Check size={12} className="text-success" /> Copied</>
          ) : (
            <><Copy size={12} /> Copy</>
          )}
        </Button>
      </div>
      {/* Code content */}
      <pre className="m-0 px-3 py-3 bg-surface-container overflow-x-auto">
        <code className="text-small font-mono text-on-surface leading-relaxed whitespace-pre">
          {code}
        </code>
      </pre>
    </div>
  )
}

/** Wrap @mentions in highlight spans for HTML content. */
function highlightMentions(html: string): string {
  return html.replace(
    /@(\w+)/g,
    '<span class="text-primary font-medium bg-primary/10 px-1 rounded">@$1</span>'
  )
}

/** Wrap @mentions in bold for markdown content. */
function highlightMentionsMarkdown(text: string): string {
  return text.replace(/@(\w+)/g, '**@$1**')
}
