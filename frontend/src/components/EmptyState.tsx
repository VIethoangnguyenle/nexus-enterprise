import { Button, Heading, Text } from './primitives'

interface EmptyStateProps {
  icon?: string
  title: string
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
}

/** Full-section empty state with icon, title, description, and optional CTA. */
export function EmptyState({ icon = '📭', title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 animate-fade-in">
      <span className="text-4xl mb-4">{icon}</span>
      <Heading as="h3">{title}</Heading>
      {description && <Text variant="body" muted className="mt-2 max-w-xs text-center">{description}</Text>}
      {action && (
        <Button onClick={action.onClick} className="mt-4">{action.label}</Button>
      )}
    </div>
  )
}
