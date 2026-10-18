import { createFileRoute } from '@tanstack/react-router'
import { SettingsScreen } from '../../../components/settings/SettingsScreen'

/** /settings: the tabs on a wide screen, Tôi on a phone. */
export const Route = createFileRoute('/_workspace/settings/')({
  component: SettingsScreen,
})
