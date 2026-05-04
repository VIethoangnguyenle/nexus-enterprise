import { createFileRoute } from '@tanstack/react-router'
import { Heading, Text } from '../../components/primitives'

export const Route = createFileRoute('/_workspace/settings')({ component: SettingsPage })

function SettingsPage() {
  return (
    <div className="animate-fade-in p-6">
      <h1 className="font-h1 text-h1 text-on-surface">Settings</h1>
      <p className="text-sm text-on-surface-variant mt-2">Workspace settings coming soon.</p>
    </div>
  )
}
