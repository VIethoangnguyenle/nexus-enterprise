# Fix Spec: System Audit Bugfix

## Scope
Fix 5 bugs found during system-wide QA audit. All frontend, no backend changes except BUG-004.

---

## BUG-001: Mobile Approvals nav dead
**Root cause**: MobileNav component likely missing onClick handler or route for Approvals.
**Fix**: Add proper route navigation to `MobileNav.tsx` for Approvals item.
**AC**:
- [ ] Clicking Approvals in mobile bottom nav navigates to /approval
- [ ] Approvals icon highlights as active when on /approval route

## BUG-002: Mobile Contacts nav dead
**Root cause**: Same as BUG-001 — missing handler for Contacts.
**Fix**: Add proper route navigation to `MobileNav.tsx` for Contacts item.
**AC**:
- [ ] Clicking Contacts in mobile bottom nav navigates to /contacts
- [ ] Contacts icon highlights as active when on /contacts route

## BUG-003: Assets traps user — no global nav
**Root cause**: Assets module uses its own layout that replaces the workspace layout sidebar entirely.
**Fix**: Ensure Assets route renders within the workspace layout OR provides a clear "back to workspace" navigation.
**AC**:
- [ ] User can navigate from Assets back to Chat/Drive/Contacts without browser back button
- [ ] Mobile: bottom nav remains visible in Assets module
- [ ] Desktop: global sidebar persists or a back button is clearly visible

## BUG-004: Approval 404 tenant schema
**Root cause**: Approval service requires tenant schema provisioning. Schema may not be applied via init.sql.
**Fix**: Add approval schema migration to data/init.sql OR handle the 404 gracefully in frontend with proper empty state.
**AC**:
- [ ] Approval module either loads data or shows proper empty state (not raw error)
- [ ] No 404 errors in normal operation

## BUG-005: Chat back button dead
**Root cause**: Mobile chat header back arrow has no onClick handler or incorrect event binding.
**Fix**: Wire the back arrow in chat header to show channel list on mobile.
**AC**:
- [ ] Clicking back arrow in mobile chat navigates back to channel list
- [ ] Works consistently (not intermittent)
