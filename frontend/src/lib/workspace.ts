/**
 * Display name for a workspace. Personal workspaces are created as
 * "user_123's workspace", which carries an internal handle; show a plain
 * phrase instead.
 */
export function workspaceDisplayName(name: string | undefined): string {
  if (!name) return 'Không gian làm việc'
  if (/^user_\d+'s\s+workspace$/i.test(name)) return 'Không gian của bạn'
  return name
}
