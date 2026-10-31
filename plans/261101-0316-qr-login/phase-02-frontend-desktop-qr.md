# Phase 02 — Frontend: desktop QR panel

Design: `design/mockups/qr-login.html` sections 1 and 2. Render the design; do not redesign.

## Files

- `frontend/src/components/auth/LoginScreen.tsx` — add the corner switch ("Mã QR" ⇄
  "Email hoặc Google", icon + text, secondary with hairline). Remember the choice in
  `localStorage` (try/catch). Default to the form below 1024 px.
- `frontend/src/components/auth/QrLoginPanel.tsx` (new) — QR, countdown, veil states
  (pending, scanned, approving, declined, expired, reconnecting), `aria-live="polite"`.
- `frontend/src/hooks/useQrLogin.ts` (new) — TanStack Query:
  - `useMutation` create → `{token, expires_in}`;
  - `useQuery` status with `refetchInterval: 2000` while visible (`refetchIntervalInBackground: false`);
  - renew automatically ~5 s before expiry, at most 5 consecutive renewals without a scan,
    then stop at "Hết hạn";
  - on `approved`: `login(access_token, sessionUser(user))` exactly like OTP verify
    (`hooks/useAuth.ts:37-43`), then navigate as the OTP path does (last workspace of that tenant,
    no workspace picker);
  - network error: keep the QR, retry silently; after 3 failures show "Mất kết nối, đang thử lại".
- `frontend/src/api/auth.ts` — `qrApi.create/status` via `publicFetch` with `credentials: 'include'`.
- QR rendering: add `qrcode` (npm), render to SVG string, error correction `H`, logo tile in
  the centre, true white quiet zone in both themes (documented exemption, as in the mockup).
  The encoded value is `${window.location.origin}/qr/${token}`.

## Tests (vitest)

- `useQrLogin`: renew cap (5), stops polling on declined/expired, calls `login` once on approved.
- `QrLoginPanel`: each state renders its copy; scanner shows name, never an ID.
- `LoginScreen`: switch toggles views and persists.

## Done when

`npm test`, `npm run lint`, `npm run build` green; screenshot of light + dark matches mockup.
