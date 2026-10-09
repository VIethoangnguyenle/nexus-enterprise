import { adminKeys } from './admin'
import { authKeys } from './auth'
import { approvalKeys } from './approval'
import { assetKeys } from './assets'
import { contactKeys } from './contacts'
import { documentKeys } from './documents'
import { driveKeys } from './drive'
import { messagingKeys } from './messaging'
import { notificationKeys } from './notifications'
import { permissionKeys } from './permissions'
import { workspaceKeys } from './workspaces'

/**
 * Every TanStack Query key in the app comes from here, so a write and the
 * invalidation that should follow it cannot drift apart: both call the same
 * factory. Never write an array literal as a key in a hook or store.
 *
 * Data that belongs to a workspace takes the workspace id as its first
 * argument; `all(wsId)`-style prefixes invalidate it as a group.
 */
export const keys = {
  admin: adminKeys,
  approval: approvalKeys,
  assets: assetKeys,
  auth: authKeys,
  contacts: contactKeys,
  documents: documentKeys,
  drive: driveKeys,
  messaging: messagingKeys,
  notifications: notificationKeys,
  permissions: permissionKeys,
  workspaces: workspaceKeys,
}
