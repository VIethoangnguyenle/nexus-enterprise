import { useSearch } from '@tanstack/react-router'
import type { DriveSearch } from '../../lib/drive-search'
import { TextsScreen } from '../documents/TextsScreen'
import { DriveScreen } from './DriveScreen'

/** Tài liệu has two bodies under one list panel: files and folders, or Văn bản. The URL says which. */
export function DriveRoute() {
  const search = useSearch({ strict: false }) as DriveSearch
  return search.view === 'texts' ? <TextsScreen /> : <DriveScreen />
}
