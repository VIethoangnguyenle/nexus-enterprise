# QA Report: System-Wide Audit
**Run**: #1 (Initial Audit)
**Date**: 2026-05-01
**Dev Server**: http://localhost:5173

## Summary
- Total checks: 24
- Passed: 17
- Failed: 7
- Severity: HIGH: 3, MEDIUM: 3, LOW: 1

## Visual Results (Desktop 1280px)
| Screen | Status | Notes |
|--------|--------|-------|
| Chat | ✅ | Layout OK, messages render, input works |
| Drive | ✅ | 3-column layout, empty state correct |
| Contacts | ✅ | Table renders, 1 member shows correctly |
| Approvals | ⚠️ | Shows 404 "tenant schema not provisioned" in server logs |
| Workplace/Assets | ✅ | Navigates but separate sidebar replaces global nav |
| Settings | ✅ | Shows in sidebar as extra panel |

## Issue List

### BUG-001: Mobile Bottom Nav — Approvals button non-functional
- **Type**: Flow
- **Severity**: HIGH
- **Screen**: Mobile bottom navigation
- **Description**: Clicking "Approvals" button in mobile bottom nav does nothing. Stays on current screen (Drive).
- **Expected**: Navigate to /approval route
- **Actual**: No response, no navigation
- **Evidence**: click_feedback_1777603981551.png — still shows Drive after clicking Approvals
- **Route to**: Dev

### BUG-002: Mobile Bottom Nav — Contacts button non-functional
- **Type**: Flow
- **Severity**: HIGH
- **Screen**: Mobile bottom navigation
- **Description**: Clicking "Contacts" button in mobile bottom nav does nothing. Stays on current screen (Drive).
- **Expected**: Navigate to /contacts route
- **Actual**: No response, no navigation
- **Evidence**: click_feedback_1777603950406.png — still shows Drive after clicking Contacts
- **Route to**: Dev

### BUG-003: Assets/Workplace module traps user — no global nav escape
- **Type**: Flow
- **Severity**: HIGH
- **Screen**: Assets module (all breakpoints)
- **Description**: When entering Workplace/Assets, the global sidebar (Chat, Drive, etc.) is completely replaced by an asset-specific sidebar. User has no way to navigate back to other modules except browser back button.
- **Expected**: Global nav should persist or drawer should be accessible
- **Actual**: Global nav disappears, replaced by asset categories
- **Evidence**: Desktop screenshots show separate sidebar; mobile bottom nav disappears entirely in Assets
- **Route to**: Dev

### BUG-004: Approval service returns 404 — tenant schema not provisioned
- **Type**: Data
- **Severity**: MEDIUM
- **Screen**: Approvals module
- **Description**: All approval API endpoints return 404 with "tenant schema not provisioned". The approval module UI shows but no data loads.
- **Expected**: Approval data should load or show proper empty state
- **Actual**: 404 errors in server logs, UI may show broken state
- **Evidence**: Server logs show multiple 404s on /api/approval/* endpoints
- **Route to**: Dev

### BUG-005: Mobile Chat back button non-functional
- **Type**: Flow
- **Severity**: MEDIUM
- **Screen**: Chat (375px)
- **Description**: The back arrow (←) in mobile chat header doesn't navigate back to channel list. Only the "Menu" button works.
- **Expected**: Back arrow navigates to channel list
- **Actual**: Click does nothing
- **Evidence**: click_feedback_1777604035557.png
- **Route to**: Dev

### BUG-006: Mobile touch targets too small
- **Type**: UI
- **Severity**: MEDIUM
- **Screen**: Multiple (375px)
- **Description**: Several mobile buttons (Menu, header icons) are 32-36px, below the 44px minimum for touch targets.
- **Expected**: All touch targets ≥ 44px
- **Actual**: Some are 32-36px
- **Route to**: Dev

### BUG-007: Settings panel overlaps Contacts content
- **Type**: UI
- **Severity**: LOW
- **Screen**: Settings (desktop 1280px)
- **Description**: Clicking Settings opens a sidebar panel that overlaps the current module content (Contacts Directory remains visible behind Settings panel), creating visual clutter.
- **Expected**: Settings should be its own view or properly overlay
- **Actual**: Partially overlaps existing content
- **Evidence**: click_feedback_1777603852473.png
- **Route to**: Dev

## Module Verdict
- **Overall System**: FAIL (3 HIGH issues)
- **Blocker count**: 3 (BUG-001, BUG-002, BUG-003)
- **Recommendation**: Fix HIGH issues first, then MEDIUM
