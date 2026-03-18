import { createContext, useContext } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

const InsidePre = createContext(false)

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
        code: ({ children }) => {
          const isBlock = useContext(InsidePre)
          return isBlock ? (
            <code className="block">{children}</code>
          ) : (
            <code className="px-1 py-0.5 rounded text-xs bg-muted border border-border font-mono text-foreground/90">
              {children}
            </code>
          )
        },
        pre: ({ children }) => (
          <InsidePre.Provider value={true}>
            <pre className="rounded-md bg-muted border border-border px-3 py-2.5 text-xs font-mono overflow-x-auto my-3">
              {children}
            </pre>
          </InsidePre.Provider>
        ),
        h2: ({ children }) => (
          <h2 className="text-sm font-semibold text-foreground mt-4 mb-2 first:mt-0">{children}</h2>
        ),
        h3: ({ children }) => (
          <h3 className="text-sm font-semibold text-foreground/80 mt-3 mb-1.5">{children}</h3>
        ),
        ul: ({ children }) => (
          <ul className="list-disc list-inside text-sm text-foreground/90 space-y-1 mb-3">{children}</ul>
        ),
        ol: ({ children }) => (
          <ol className="list-decimal list-inside text-sm text-foreground/90 space-y-1 mb-3">{children}</ol>
        ),
        li: ({ children }) => <li className="leading-relaxed">{children}</li>,
        em: ({ children }) => <em className="italic text-foreground/70">{children}</em>,
      }}
    >
      {content}
    </ReactMarkdown>
  )
}
