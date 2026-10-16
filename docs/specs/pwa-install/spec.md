# pwa-install

## Purpose
Nexus can be added to a phone's home screen and opened as an app, keeps its shell when the network drops, and tells the
person plainly when it is offline or when a new version is waiting. It opens from a short-lived copy of the last
session's navigator data kept on the device, which is wiped when the session ends.

## Requirements

### Requirement: Installable
The app SHALL serve `/manifest.webmanifest` (name and short name "Nexus", `start_url` "/", `display` "standalone",
theme and background colour from the light `--color-base` token, icons 192 and 512 plus maskable 192 and 512) and a
service worker at `/sw.js`, whose scope is "/". `index.html` SHALL link the manifest and the 180px apple touch icon,
declare `theme-color` for light and dark, and the web-app-capable metas that iOS Safari reads.

### Requirement: The service worker caches the shell, never data
The service worker SHALL precache the build's HTML, JavaScript, CSS, fonts and icons, and answer any page navigation
with the cached shell (`/index.html`) — except paths under `/api/`. It SHALL NOT cache, intercept or answer any `/api/*`
request or the WebSocket: messages, documents and every other response come from the network or fail. The service worker stores
nothing for reading offline; the only copy of data on the device is the one in the next requirement.

#### Scenario: Offline navigation
- **WHEN** the device is offline and a page of the app is opened
- **THEN** the cached shell is served, the person stays signed in (the session refresh is retried when the network
  returns) and the connection bar says so (data is not shown)

#### Scenario: API while offline
- **WHEN** the app calls `/api/...` with no network
- **THEN** the request fails; no cached response is returned

### Requirement: Connection bar
One bar under the top bar SHALL appear after 2 seconds of the WebSocket being down or the browser reporting `offline`:
"Mất kết nối. Đang thử kết nối lại…" with a "Thử lại" action that reconnects. When the connection returns it SHALL read
"Đã kết nối lại" in the success colour for 2 seconds and then disappear. A drop shorter than 2 seconds shows nothing.

### Requirement: Updates never interrupt typing
A new service worker SHALL wait. The app SHALL show a floating bar "Có bản mới của Nexus" with "Để sau" and "Tải lại":
"Tải lại" activates the worker and reloads; "Để sau" hides the bar for this visit and cancels a reload that was waiting.
The reload SHALL wait while unsent work exists or a request is in flight (a send, a save, a fetch). Unsent work is a text
box, text area or editor the person typed into that still holds text, plus anything typed in the last 3 seconds (a
document between keystroke and autosave); a field that was pre-filled and never edited holds nothing. While unsent text
exists the action reads "Tải lại sau khi gửi" and the reload happens once the text is gone and nothing is in flight.
The worker is never activated without the person's action. Next to a composer the bar sits at the top of the screen so
it never covers the send button. The bar and the connection bar are announced to screen readers through live regions
that exist before their text appears. If another tab already activated the new worker, "Tải lại" simply reloads.

#### Scenario: Update with a draft
- **WHEN** a new version is waiting and the composer has text
- **THEN** the page is not reloaded; pressing "Tải lại sau khi gửi" reloads it after the message is sent

#### Scenario: Changing one's mind
- **WHEN** the person pressed "Tải lại sau khi gửi" and then "Để sau"
- **THEN** the page is not reloaded, even after the text is sent

### Requirement: The app opens from a copy kept on the device
The app SHALL keep a copy of the TanStack Query cache in IndexedDB (database `nexus-device-cache`) so that the next
start shows Trang chủ and the conversation list from it while fresh data loads. The copy holds only: the chat state and
the queries seeded from it (channels, direct messages, unread counts, conversation activity, sections, mention and
thread counts), the contact directory, `/me` and the workspace list, the newest page (at most 50 messages, newest
first, nothing unsent) of the 20 conversations with the most recent message, and the delta-sync cursors, written and
restored together with the cache so the replay after a start begins from what the cache shows. It SHALL NOT hold access
or refresh tokens, one-time codes, file contents, thumbnails, download or presigned URLs, or any other query. The entry
is keyed by person and workspace and records its owner; restoring reads only the entry of the person the app was left
open for, and an entry whose owner differs is deleted unread. The copy is written at most every 1.5 seconds while the
cache changes and when the page is hidden, and only once the session's person and workspace are known.

The whole store SHALL be emptied when: the person signs out; any request ends the session (a 401 that the refresh does
not repair, or the refresh refused with 401/403 — a failing server, 5xx, is not an answer about the session); the
workspace is switched; the copy is older than 7 days; the copy was written by another build (the build id is the buster);
or it cannot be read. A save that was waiting when the wipe came SHALL NOT write afterwards. If the session turns out to
speak for another workspace than the copy, the restored data and cursors are dropped. Starting never waits more than
1.5 seconds for the device, and a device without IndexedDB simply has no copy.

Because the copy is business data on a possibly shared machine, these wipe conditions and the owner check are the
whole boundary; the copy is not encrypted.

#### Scenario: Next start
- **WHEN** the app is opened again within 7 days by the same person with the same build
- **THEN** the chat state, contacts and recent messages are in the cache before any request is answered, the restored
  queries count as stale and are fetched again, and the cursors resume from the saved values

#### Scenario: Another person on the same device
- **WHEN** person B signs in on a device where person A used the app
- **THEN** A's copy was already wiped at sign-out, and even if it were not, B's start never reads an entry owned by A

#### Scenario: Session ends
- **WHEN** the person signs out, the refresh is refused, or a request returns 401 that refresh cannot repair
- **THEN** the store is empty and nothing is written back from a save that was pending

#### Scenario: Old or foreign copy
- **WHEN** the copy is older than 7 days or written by another build
- **THEN** it is deleted and the app starts as it did before any copy existed

### Requirement: Production only, and always fresh
The worker SHALL be registered by production builds only; the dev server (`make dev`) never installs one. The edge
SHALL serve `/sw.js` and `/manifest.webmanifest` with `Cache-Control: no-cache`, so an installed app finds a new
version on its next open.

## Status
Matches code, with these gaps: the composer is not yet locked while offline; the in-screen notices in Danh bạ and Tài
liệu still appear next to the bar; "Thử lại" calls the socket store's `connect` directly, which can open a second
socket while one is connecting (the store needs a `retryNow`); the manifest carries only the light theme colour (the
`theme-color` metas follow the theme in the browser); uploads that are not TanStack Query requests are not seen as in
flight; the standalone `frontend/nginx.conf` does not yet mirror the no-cache rule for `sw.js`.
