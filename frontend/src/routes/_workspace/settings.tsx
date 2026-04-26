import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_workspace/settings')({ component: SettingsPage })

function SettingsPage() {
  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <h1 className="page-title">Settings</h1>
      <p className="text-muted">Workspace settings coming soon.</p>
    </div>
  )
}
