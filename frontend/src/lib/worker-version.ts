// The worker OneSignal registers (public/OneSignalSDKWorker.js) is one line that imports /sw.js.
// A browser decides a worker changed by comparing that registered script; Safari does not compare
// what it imports, so with the same bytes on every deploy an installed iPhone app never updated.
// The build writes its own id into the import, so the registered script differs per deploy.

const IMPORT = /importScripts\(\s*(['"])\/sw\.js\1\s*\)/

/** The worker source with `/sw.js` imported as `/sw.js?v=<version>`. */
export function versionWorkerImport(src: string, version: string): string {
  if (!IMPORT.test(src)) {
    throw new Error('OneSignalSDKWorker.js no longer imports /sw.js; the build cannot version it')
  }
  return src.replace(IMPORT, `importScripts('/sw.js?v=${encodeURIComponent(version)}')`)
}
