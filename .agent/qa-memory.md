# QA Memory — NGAC Nexus Enterprise Platform

> Persistent runtime QA data across sessions. Always read this before testing.
> Format specifications live in `.agent/skills/agent-qc/SKILL.md`.

## Last Updated: 2026-05-01

---

## Known Issues (Active)

| ID | Module | Severity | Description | Root Cause | First Found |
|----|--------|----------|-------------|------------|-------------|
| BUG-001 | Notifications | P0 | `GET /api/notifications/unread-count` returns 500 ISE | `notifSt` is `nil` in `rest.NewHandler()` — main.go L204 | 2026-04-30 |
| BUG-002 | Asset Mgmt | P0 | Dashboard shows ErrorState — missing `/assets/summary` endpoint | Backend has no summary route; frontend calls `useAssetSummary()` | 2026-04-30 |
| BUG-003 | Messaging | P0 | Messages typed and sent but don't appear in chat area | Query cache invalidation or WebSocket broadcast issue | 2026-04-30 |
| BUG-004 | Asset Mgmt | P0 | Entire module shows "Not Found" on all sub-pages | Cascading failure from BUG-002 in parent layout | 2026-04-30 |
| BUG-005 | Navigation | P1 | "New Project" button does nothing | No click handler wired | 2026-04-30 |
| BUG-006 | Header | P1 | Bell, Help, Settings icons non-functional | No onClick handlers | 2026-04-30 |
| BUG-007 | Drive | P1 | Folder SIZE and MODIFIED show "–" | Frontend not formatting protobuf timestamps | 2026-04-30 |
| UX-001 | Messaging | P2 | Empty channel has no welcome message | No empty state component | 2026-04-30 |
| UX-005 | Layout | P2 | Sidebar doesn't collapse on mobile | No responsive drawer | 2026-04-30 |

## Resolved Issues

*(None yet)*

---

## Environment Notes

- **Ports:** Auth:8180, Workspace:8181, Document:8182, Messaging:8183, Asset:8184, Drive:8185, Approval:8186
- **Vite proxy:** Routes `/api/auth` → 8180, `/api/workspaces` → 8181, etc.
- **DB:** Docker postgres, seeded with test workspace + user
- **Dev OTP:** Code is always `999999`
- **Test user:** `test@company.com`
- **Test workspace:** `test.company's Workspace` (ID: `784d97f9-643e-4a1b-8db5-028e9e53a1b2`)
