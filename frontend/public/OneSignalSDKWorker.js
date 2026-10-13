/* global importScripts */
// OneSignal registers this URL (its dashboard setting wins over serviceWorkerPath in code).
// It is the app worker: offline shell, update prompt and the OneSignal push handlers.
importScripts('/sw.js')
