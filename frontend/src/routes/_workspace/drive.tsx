import { createFileRoute } from '@tanstack/react-router'
import { DriveScreen } from '../../components/drive/DriveScreen'
import { validateDriveSearch } from '../../lib/drive-search'

// The open folder and view are part of the URL (`?folder=`, `?view=shared`), so
// reload, Back/Forward and links keep the place. `?ws=` is kept by the layout.
export const Route = createFileRoute('/_workspace/drive')({
  validateSearch: validateDriveSearch,
  component: DriveScreen,
})
