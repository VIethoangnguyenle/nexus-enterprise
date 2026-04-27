import { createFileRoute } from '@tanstack/react-router'
import { Heading, Text } from '../../components/primitives'

export const Route = createFileRoute('/_workspace/settings')({ component: SettingsPage })

function SettingsPage() {
  return (
    <div className="animate-fade-in">
      <Heading as="h2">Settings</Heading>
      <Text variant="body" muted className="mt-2">Workspace settings coming soon.</Text>
    </div>
  )
}
