import { useCallback, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Copy, Check } from 'lucide-react'
import { Button } from '../primitives'

/** Markdown message content with a custom code renderer. Loaded on demand by `MessageContent`. */
export function MarkdownBody({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        code: CodeBlock,
      }}
    >
      {highlightMentionsMarkdown(content)}
    </ReactMarkdown>
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
      <code className="bg-surface-container px-1.5 py-0.5 rounded text-sm text-primary font-mono" {...props}>
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
        <code className="text-sm font-mono text-on-surface leading-relaxed whitespace-pre">
          {code}
        </code>
      </pre>
    </div>
  )
}

/** Wrap @mentions in bold for markdown content. */
function highlightMentionsMarkdown(text: string): string {
  return text.replace(/@(\w+)/g, '**@$1**')
}
