import { createFileRoute } from '@tanstack/react-router'
import { SettingsSectionScreen } from '../../../components/settings/SettingsSectionScreen'

/** /settings/<section>: one settings page, the same one from Tôi and from a module's ⋯. */
export const Route = createFileRoute('/_workspace/settings/$section')({
  component: SettingsSectionScreen,
})
