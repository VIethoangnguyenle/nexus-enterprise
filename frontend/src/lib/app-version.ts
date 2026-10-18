/**
 * The build the person is running, `2026.10.18 · 6368e8e` (date and short commit),
 * stamped in at build time through VITE_APP_VERSION. A local run has none: "dev".
 */
export function appVersion(): string {
  const raw: unknown = import.meta.env.VITE_APP_VERSION
  return typeof raw === 'string' && raw.trim() ? raw.trim() : 'dev'
}

/** The quiet "Phiên bản …" line shown in Cài đặt and the account menus. */
export function appVersionLabel(): string {
  return `Phiên bản ${appVersion()}`
}
