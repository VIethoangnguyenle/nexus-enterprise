# Phone composer jumps to top when keyboard opens

## Root cause
`frontend/src/lib/app-height.ts` (commit 6e09f58) sized the shell (`h-app`, `--app-height`) from `visualViewport.height` only, on `resize`. On iOS Safari the layout viewport is not resized by the keyboard; instead iOS scrolls the page so the focused field is visible, so `visualViewport.offsetTop` becomes about the keyboard height. The shell stayed at document top (y=0) with height = visible height, so its bottom (the composer) landed at screen y = height - offsetTop, near the top of the screen; header scrolled out above. Hook never read `offsetTop` and never listened to the visualViewport `scroll` event (which is the only event fired when offsetTop changes without a size change).
Android Chrome: `interactive-widget=resizes-content` resizes the layout viewport, offsetTop stays 0, so unaffected. Hypotheses eliminated by reading: fixed-position composer (ChatEditor/SpaceView are in-flow in the shell), scrollIntoView on focus (none found in the composer path).

## Fix
- `lib/app-height.ts`: also publish `--app-top` = `visualViewport.offsetTop`, listen to `scroll` as well as `resize`, clean up both.
- `index.css`: `h-app` adds `translate: 0 var(--app-top, 0px)`, so the shell (and SidePanel's `max-lg:fixed ... h-app`) sits exactly on the visible viewport, composer above the keyboard, header at the top of the visible area.
- Test first: `lib/app-height.test.ts` new case (offsetTop change with scroll event) failed before (`--app-top` empty), passes after.

## Gates
vitest full: 177 files / 2255 tests pass. eslint clean. typecheck:diff: no file worse than baseline. Build skipped.

## Verified / not verified
Verified: hook unit logic with simulated visualViewport resize+scroll; gates.
Not verified: real iOS behaviour. Playwright WebKit is not installed here (only chromium), and Chromium does not scroll the page on focus, so the iOS scroll-to-reveal could not be reproduced; root cause rests on documented iOS visualViewport behaviour plus the code reading. Not checked on a device: whether `translate` on the shell visually jitters during iOS's keyboard animation (events fire per frame, should track), and the SidePanel fixed variant. Please confirm on the phone after deploy.

## Unresolved
- If iOS still shows a gap/jitter, fallback is `position: fixed; top: 0` shell with the same translate, or locking body scroll (`overflow: hidden`) on conversation routes.
