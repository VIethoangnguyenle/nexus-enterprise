import { Button, Heading, Text } from './primitives'

interface ErrorStateProps {
  title?: string
  message?: string
  onRetry?: () => void
}

/** Displays a full-section error message with optional retry action. */
export function ErrorState({ title = 'Something went wrong', message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 animate-fade-in">
      <span className="text-4xl mb-4">⚠️</span>
      <Heading as="h3">{title}</Heading>
      {message && <Text variant="body" muted className="mt-2 max-w-md text-center">{message}</Text>}
      {onRetry && <Button onClick={onRetry} className="mt-4">Retry</Button>}
    </div>
  )
}
