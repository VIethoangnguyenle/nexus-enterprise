---
title: "Đăng nhập bằng mã QR (kiểu Lark)"
description: "Desktop sign-in shows a single-use QR; a phone already signed in to Nexus scans it (in-app scanner or plain camera), sees which computer is asking, and approves. The desktop then gets a session for the same user and the tenant the phone is using."
status: in-review
priority: P2
effort: 22h
branch: feat/qr-login
base: main (36edb19)
tags: [auth, frontend, mobile, security, design-first]
created: 2026-11-01
blockedBy: []
blocks: []
mockup: design/mockups/qr-login.html
specs:
  new: []
  modified: [tenant-auth-flow, sign-in-screens]
---

# Đăng nhập bằng mã QR

## Outcome

A person who is signed in to Nexus on their phone can sign in on a computer without typing:
the computer shows a QR, the phone scans it, the phone shows "Chrome trên Windows wants to
sign in as Lê Thị Hoa · NovaPay", the person taps Đăng nhập, and the computer lands in that
tenant's last workspace within about two seconds.

## Decisions already made (by Zane, 2026-11-01)

- Scanning works two ways: the QR is a URL (`https://<origin>/qr/<token>`), so the phone's
  ordinary camera opens it in the browser; and a "Quét mã QR" scanner inside the mobile web
  (in the + sheet, like Lark).
- The computer signs in to the **tenant currently open on the phone**. No tenant picker on the
  phone; the person switches later on the computer.
- Mockup first, reviewed by Zane, then implementation without further stops.

## Constraints

- No native app exists. The phone side is the mobile web; camera access is `getUserMedia`.
- Sessions live in Redis (`refresh:*`, `refresh_session:*`); there is no session table and no
  migration is needed. QR state is Redis-only, TTL-bound.
- The auth service has no realtime channel and the signed-out page cannot use the messaging WS,
  so the desktop **polls** (every 2 s while the tab is visible).
- Every sign-in path issues the cookie through `rest/session.go` `issueSession`; QR must too.
- No NGAC change: QR login mints a session for an identity that already holds a membership; it
  creates no node, assignment or association. Membership is re-checked at approval.
- No raw identifier on screen (CLAUDE.md §5): the phone shows browser + OS + time, never IP,
  token or device id.

## Non-goals

- A device/session list ("Thiết bị đăng nhập") or remote sign-out of one device.
- IP geolocation on the confirm screen.
- Desktop-to-phone (reverse) QR, or QR sign-in from a tablet to itself.
- Native push "approve on your phone" without scanning.

## Threat model (what the design protects)

| Risk | Mitigation |
|---|---|
| Attacker sends victim their own desktop QR (QR-jacking) | Confirm screen names the requesting browser/OS and time, carries a permanent warning, and needs an explicit tap. Same mitigation Lark/WeChat use. |
| Someone who photographs the QR claims the session | The session is released only to the browser that created the QR: creation sets an HttpOnly `qr_poll` cookie (Path `/api/auth/qr`, SameSite Strict); status polling without that cookie never returns a session. |
| Token guessing / enumeration | 32 random bytes, stored as sha256; public endpoints rate-limited; every unusable-token case returns the same `qr_unavailable`. |
| Replay | Single use: approval stores the identity; the claiming poll does an atomic GetDel. TTL 120 s. |
| Member removed between scan and approve | Approval re-runs `activeMemberRow` for the phone's tenant; failure → `tenant_access_denied`, phone shows the "không đăng nhập được vào …" screen. |
| Token in logs | Token travels in request bodies (scan/approve/decline) and the URL path of a SPA route only; REST handlers never log it. |

## Phases

| # | Phase | Effort | Depends |
|---|---|---|---|
| 01 | [Backend: QR login domain + REST](phase-01-backend-qr-login.md) | 8h | — |
| 02 | [Frontend: desktop QR panel](phase-02-frontend-desktop-qr.md) | 5h | 01 |
| 03 | [Frontend: phone confirm route + scanner](phase-03-frontend-phone-scan-confirm.md) | 6h | 01 |
| 04 | [Specs, design doc, verification](phase-04-specs-and-verification.md) | 3h | 02, 03 |

02 and 03 touch different files and can run in parallel after 01.

## Acceptance criteria

1. Desktop `/login` has a "Mã QR" corner switch; the QR view renders a real, scannable QR
   (error correction H, logo in the centre) and refreshes itself before expiry, at most 5 times.
2. Scanning with the phone's camera app opens `/qr/<token>`; scanning from + → Quét mã QR does
   the same. A non-Nexus QR shows "Đây không phải mã đăng nhập Nexus" and keeps scanning.
3. On scan, the desktop shows the scanner's avatar and name; the phone shows browser + OS + time,
   the account and tenant, and the anti-phishing warning.
4. Approve → the desktop receives `access_token` + refresh cookie for the phone user and
   `claims.TenantID == phone's tenant`, and lands in that tenant without the workspace picker.
5. Decline / close → desktop shows Bị từ chối; expiry → Hết hạn; both offer "Lấy mã mới".
6. A second browser that knows the token but lacks the `qr_poll` cookie can never obtain a
   session (test). A used, expired or unknown token returns the same error (test).
7. A member removed from the tenant before approval cannot complete it (test, deny case).
8. Phone not signed in → sign-in, then back to the same `/qr/<token>` confirm screen.
9. `make build-check`, `make check-layering`, `make test s=auth`, `npm test`, `npm run lint`,
   `npm run build` all pass; specs `tenant-auth-flow` and `sign-in-screens` describe the flow.
