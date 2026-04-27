import type { HTMLAttributes, ReactNode } from 'react'

interface CardPartProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
}

function CardRoot({ children, className = '', ...props }: CardPartProps) {
  return (
    <div
      className={`bg-bg-tertiary border border-border rounded-[var(--radius-md)] overflow-hidden ${className}`}
      {...props}
    >
      {children}
    </div>
  )
}

function CardHeader({ children, className = '', ...props }: CardPartProps) {
  return (
    <div
      className={`flex items-center justify-between px-4 py-3 border-b border-border ${className}`}
      {...props}
    >
      {children}
    </div>
  )
}

function CardBody({ children, className = '', ...props }: CardPartProps) {
  return (
    <div className={`p-4 ${className}`} {...props}>
      {children}
    </div>
  )
}

function CardFooter({ children, className = '', ...props }: CardPartProps) {
  return (
    <div
      className={`flex items-center gap-2 px-4 py-3 border-t border-border ${className}`}
      {...props}
    >
      {children}
    </div>
  )
}

/** Compound card component. Usage: <Card><Card.Header>...</Card.Header><Card.Body>...</Card.Body></Card> */
export const Card = Object.assign(CardRoot, {
  Header: CardHeader,
  Body: CardBody,
  Footer: CardFooter,
})
