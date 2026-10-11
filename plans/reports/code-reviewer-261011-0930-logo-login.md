# Code review: Nexus logo (direction A) + animated sign-in

Read-only review of the uncommitted work listed in
`plans/reports/fullstack-developer-261011-0800-logo-login.md`. Chat files changed in parallel are out of scope and were not reviewed.

## Scope
- Files: `frontend/src/components/brand/{NexusMark,NexusLogo}.tsx`, `nexus-mark.css`; `frontend/src/components/auth/{auth-motion.ts,auth-motion.css,AuthGraph.tsx,AddressFlight.tsx,AuthShell.tsx,LoginScreen.tsx,CodeEntry.tsx}` plus their tests; `primitives/OtpInput.tsx` and its test; `frontend/index.html`; `frontend/public/*`; `DESIGN.md` (Logo section and the §7 sign-in exception); `docs/specs/sign-in-screens/spec.md`.
- Checks I ran from `frontend/`:
  - `npx vitest run src/components/auth src/components/brand src/components/primitives`: 247 passed, 0 failed.
  - `npx eslint` on the same three directories: 0 issues.
  - `tsc --noEmit -p tsconfig.json`: 7 errors in total, none of them in the reviewed files.
- Icons: `file` reports `favicon-16` 16x16, `favicon-32` 32x32, `apple-touch-icon` 180x180 (RGB, opaque, which is correct for iOS), `icon-192/512` (RGBA), and `icon-maskable-192/512` (RGB, full bleed). I opened maskable-512 and favicon-32: the mark is the real one, not a placeholder, and the maskable mark sits inside the safe zone.

## Overall
The work is solid and stays in scope. These all hold up:
- Reduced motion comes from `useMotionPresets`, which covers both the OS setting and the in-app "Luôn bật", and every new animation follows it.
- Only `transform` and `opacity` are animated.
- Both `sessionStorage` reads and writes are wrapped in try/catch.
- The graph uses no JS timer: it plays once with `forwards`, pauses on `visibilitychange`, and never runs on phones because the brand pane is `display:none` there.
- `AddressFlight` cancels its WAAPI animation on unmount, and `CodeEntry` clears its hand-off timer.
- No IDs or secrets are rendered. The only user data shown is the person's own email.
- The logo has a text alternative: the lockup name, with the mark `aria-hidden`. A standalone mark gets `role="img"` with the name "Nexus".
- The Google button, redirects (`afterSignIn`, the `ws` carry-over) and the OTP request/verify calls are unchanged.

There is one real behaviour regression, caused by the new 700ms success hold.

## Critical
None.

## High
None.

## Medium

### M1. The success hold leaves a 700ms window where the person is signed in but can be stranded on /login
- Where: `frontend/src/components/auth/CodeEntry.tsx:106-110` (`landed` uses a `setTimeout` before `done`), together with `frontend/src/hooks/useAuth.ts:41` and `frontend/src/components/auth/LoginScreen.tsx:210,219` (back and "Đổi email" buttons).
- Cause: the `useVerifyOTP` hook-level `onSuccess` calls `login(access_token, user)` as soon as the server answers. Navigation (`done` → `navigate(afterSignIn…)`) now waits `SUCCESS_HOLD_MS` = 700ms. During that window:
  - The mobile "Quay lại" IconButton and the "Đổi email / Đổi số" link are still enabled.
  - "Gửi lại" is still enabled whenever the cooldown has run out.
- Failure scenario 1:
  1. The person types the right code, then taps "Đổi email" within 0.7s. They meant to fix a typo they noticed, or simply double-tapped.
  2. `CodeStep` unmounts and the unmount effect clears the timer, so `done` never runs.
  3. The auth store now holds a valid token, but the screen shows the identity step. `/_auth` deliberately has no guard (see the comment in `routes/_auth.tsx`), so nothing redirects.
  4. If they sign in again with another address, the first session is silently overwritten.
  5. If they reload, `beforeLoad` throws them into the app as the first account.
  6. Before this change, `done` ran synchronously in `onSuccess`, so this window did not exist.
- Failure scenario 2:
  1. The person clicks "Gửi lại" during the hold.
  2. A new OTP is requested and emailed, and `verifying.reset()` resets the boxes.
  3. `verified` stays true, so the screen still says "Đã xác minh.", and the old timer navigates anyway. The new code was sent for nothing.
- The new test `CodeEntry.test.tsx` "does not move on if the screen is left while the badge is landing" asserts this behaviour as intended. It treats the half-signed-in state as correct.
- Fix (smallest): once `verified` is true, the screen should only go forward.
  - Disable "Gửi lại" while `verified` (`disabled={verified || resendLocked || …}`).
  - Add an `onVerifying?: () => void` prop that `CodeEntry` calls in `landed`. `CodeStep` uses it to set a `leaving` flag that disables `onBack` (both buttons).
  - Alternative: if the component unmounts while the timer is pending, run `done(res)` instead of dropping it. Update the test to match whichever option is chosen.

## Low

### L1. The flying address likely lands about 7px below the text, then snaps up
- Where: `frontend/src/components/auth/AddressFlight.tsx:43-50`, together with `auth-motion.css:15-27`.
- Cause:
  - The target `<b>` sits inside the second child of `.auth-rise`. That child runs `auth-rise-in` (`translateY(8px)`, `backwards` fill, delay 160+40ms, duration 220ms).
  - `getBoundingClientRect()` is read in `useLayoutEffect` during that delay, so the measured `to.top` includes the 8px rise offset. The code compensates for the motion `x` shift (`shift`) but not for the rise.
  - The flight ends at 60+280 = 340ms. The paragraph is still about 1px from rest then, and its final rest is 8px higher than measured. When `setFlying(false)` reveals the real text, the line visibly jumps up.
- I could not confirm this in a browser: the screenshots are static and jsdom has no layout. It needs a frame-by-frame check.
- Fix: add the rise offset to `dy` (export the 8px as a constant next to `STEP_SHIFT`), or take the address paragraph out of the rise while `flying`.

### L2. The flight's start point can be stale on phones
- Where: `LoginScreen.tsx:105-109`.
- The rect is taken at submit time, before the network round-trip. With `interactive-widget=resizes-content`, the on-screen keyboard closing between submit and success moves the field. Because the flyer is `position: fixed` with the old coordinates, it starts from where the field used to be.
- Fix: re-measure is not possible once the step has exited. Accept the cosmetic glitch, or skip the flight when `visualViewport.height` changed since submit.

### L3. The logo can assemble again when crossing the `lg` breakpoint
- Where: `AuthShell.tsx` (the mobile `NexusLogo className="lg:hidden"` and the brand pane `hidden lg:grid`, both with `data-assemble`).
- CSS animations restart when an element goes from `display:none` to displayed. Rotating a tablet or resizing the window across 1024px while the first sign-in screen of the session is open replays the assembly on the other mark. On mount this breaks the "once per session" decision only in this edge case.
- Fix: drop `data-assemble` after `animationend`, or accept it.

### L4. Some tests are weak
- `LoginScreen.test.tsx` "the parts of the form come in one after another" only checks that `.auth-rise` exists. It proves nothing about the order or the stagger, so it is close to a phantom test. Either assert the `--i` / `nth-child` mapping or drop it.
- The CodeEntry hold test encodes M1 (see above).

## Informational
- The success hold adds about 0.7s to every sign-in. DESIGN.md §7 now documents it, so it is consistent with the approved design. Make sure the user accepted the added latency explicitly (report Deviation 6).
- The `OtpInput` primitive now has the pop, jolt and badge everywhere. Its only other user is `VerifyEmailNote` (workspace-select and create-workspace), which also sits inside `AuthShell`, so it stays within the "sign-in screens" exception. If `OtpInput` is ever used inside the app shell, the §7 exception will leak; consider an opt-in prop then.
- The step transition uses `AnimatePresence mode="wait"` instead of the mockup's overlap (Deviation 4), so there is a brief moment with no address text visible. This is a documented, intentional trade.
- There is no `manifest.webmanifest` yet. The icons are ready for one.
- Out of scope but still visible: "Nexus Hub" remains in `ProfileTab.tsx`.

## Checklist
- Concurrency/async: M1 (timer vs. unmount vs. auth store). AddressFlight guards its own unmount with `live`.
- Error boundaries: the verify `onError` path is unchanged. A failed flight (`finished` rejects) still calls `onDone`.
- API contracts: the `OtpInput` props are unchanged and the `status` values are unchanged.
- Reduced motion: honoured in the mark, graph (not rendered), flight (skipped), steps (120ms fade), OTP (fades only), form rise (120ms fade) and hold (240ms).
- Data exposure: none. Only the person's own email is shown, and the flyer is `aria-hidden`.

## Recommended actions
1. Fix M1: lock the back, change and resend controls once the code is verified, or navigate on unmount. Update the CodeEntry test to match.
2. Check L1 frame by frame in a browser, and add the 8px rise offset to `dy` if the snap is confirmed.
3. Strengthen or remove the phantom rise test (L4).

## Unresolved questions
- Did the user explicitly accept the extra ~0.7s on every sign-in (Deviation 6)?

Status: DONE_WITH_CONCERNS
Summary: The logo, icons and sign-in motion are well built: reduced motion is honoured, only transform/opacity animate, timers and storage are handled safely, and scoped tests, lint and types are clean. One medium regression: during the new 700ms success hold the person is already signed in but can still go back or resend, which can leave them signed in on /login with no redirect.
Concerns/Blockers: M1 should be fixed before merge. L1 (about 7px snap when the address lands) needs a browser check.
