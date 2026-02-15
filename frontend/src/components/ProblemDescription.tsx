import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface Props {
  content: string
}

export function ProblemDescription({ content }: Props) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        p: ({ children }) => (
          <p className="text-sm leading-relaxed text-foreground/90 mb-3 last:mb-0">{children}</p>
        ),
        strong: ({ children }) => (
          <strong className="font-semibold text-foreground">{children}</strong>
        ),
        code: ({ children, className }) => {
          const isBlock = !!className
          return isBlock ? (
            <code className="block">{children}</code>
          ) : (
            <code className="px-1 py-0.5 rounded text-xs bg-muted border border-border font-mono text-foreground/90">
              {children}
            </code>
          )
        },
        pre: ({ children }) => (
          <pre className="rounded-md bg-muted border border-border px-3 py-2.5 text-xs font-mono overflow-x-auto my-3">
            {children}
          </pre>
        ),
        ul: ({ children }) => (
          <ul className="list-disc list-inside text-sm text-foreground/90 space-y-1 mb-3">{children}</ul>
        ),
        li: ({ children }) => <li className="leading-relaxed">{children}</li>,
      }}
    >
      {content}
    </ReactMarkdown>
  )
}
