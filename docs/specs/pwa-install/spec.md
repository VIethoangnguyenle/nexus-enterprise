# pwa-install

## Purpose
Nexus can be added to a phone's home screen and opened as an app, keeps its shell when the network drops, and tells the
person plainly when it is offline or when a new version is waiting — without ever keeping their data on the device.

## Requirements

### Requirement: Installable
The app SHALL serve `/manifest.webmanifest` (name and short name "Nexus", `start_url` "/", `display` "standalone",
theme and background colour from the light `--color-base` token, icons 192 and 512 plus maskable 192 and 512) and a
service worker at `/sw.js`, whose scope is "/". `index.html` SHALL link the manifest and the 180px apple touch icon,
declare `theme-color` for light and dark, and the web-app-capable metas that iOS Safari reads.

### Requirement: The app shell is cached, data never
The service worker SHALL precache the build's HTML, JavaScript, CSS, fonts and icons, and answer any page navigation
with the cached shell (`/index.html`) — except paths under `/api/`. It SHALL NOT cache, intercept or answer any `/api/*`
request or the WebSocket: messages, documents and every other response come from the network or fail. Nothing is stored
for reading offline.

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
