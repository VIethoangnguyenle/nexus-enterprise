import { Button, Heading, Text } from './primitives'
import { AlertTriangle } from 'lucide-react'

interface ErrorStateProps {
  title?: string
  message?: string
  onRetry?: () => void
}

/** Displays a full-section error message with optional retry action. */
export function ErrorState({ title = 'Something went wrong', message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 animate-fade-in">
      <AlertTriangle size={40} color="#f59e0b" strokeWidth={1.5} className="mb-4" />
      <Heading as="h3">{title}</Heading>
      {message && <Text variant="body" muted className="mt-2 max-w-md text-center">{message}</Text>}
      {onRetry && <Button onClick={onRetry} className="mt-4">Retry</Button>}
    </div>
  )
}
