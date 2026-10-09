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

/**
 * `validateSearch` for every route that renders workspace data: declares
 * `?ws=` so the router knows about it instead of each page reading
 * `window.location`. The URL parser turns a numeric-looking id into a number,
 * so it is put back to text; anything that is not an id is dropped, which
 * lets `useActiveWorkspace` fall back to the user's own workspace.
 */
export function validateWorkspaceSearch(search: Record<string, unknown>): { ws?: string } {
  const { ws } = search
  if (typeof ws === 'string' && ws) return { ws }
  if (typeof ws === 'number' && Number.isFinite(ws)) return { ws: String(ws) }
  return {}
}
