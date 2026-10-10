import { createFileRoute } from '@tanstack/react-router'
import { SettingsScreen } from '../../components/settings/SettingsScreen'
import { validateSettingsSearch } from '../../lib/settings-search'

// The workspace and the open tab are part of the URL: /settings is Hồ sơ,
// ?tab=workspace and ?tab=giao-dien are the others.
export const Route = createFileRoute('/_workspace/settings')({
  validateSearch: validateSettingsSearch,
  component: SettingsScreen,
})
