// Stands in for vite-plugin-pwa's `virtual:pwa-register`, which exists only when the PWA plugin runs.
export function registerSW(): (reloadPage?: boolean) => Promise<void> {
  return () => Promise.resolve()
}
