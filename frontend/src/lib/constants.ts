/** Lifecycle state → color mapping for asset state badges. */
export const ASSET_STATE_COLORS: Record<string, string> = {
  requested: '#f59e0b',
  approved: '#10b981',
  assigned: '#3b82f6',
  in_use: '#8b5cf6',
  returned: '#6b7280',
  disposed: '#ef4444',
}

/** Request status → color mapping for asset request badges. */
export const REQUEST_STATUS_COLORS: Record<string, string> = {
  pending: '#f59e0b',
  approved: '#10b981',
  rejected: '#ef4444',
}
