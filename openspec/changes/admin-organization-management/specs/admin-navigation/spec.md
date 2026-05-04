# Admin Navigation

## User Stories

### Story 1: Access Admin Module
As an **organization admin/owner**,
I want to navigate to an admin section from the sidebar,
so that I can manage my organization structure.

**Acceptance Criteria:**
- [ ] AppSidebar shows an "Admin" icon/item at the bottom (gear or settings icon)
- [ ] Clicking "Admin" navigates to `/admin`
- [ ] Admin section has a left sidebar with 3 tabs: Organization, Users, Roles
- [ ] Default view when entering `/admin` is the Organization tab
- [ ] Admin layout is full-width (no ListPanel, similar to approval/contacts layout)
- [ ] Only workspace owners/admins see the Admin sidebar item
- [ ] Non-admin users do NOT see the Admin sidebar item

**Proto mapping:** No new RPC needed — uses existing ListWorkspaces + client-side role check

### Story 2: Admin Sub-Navigation
As an **organization admin**,
I want to switch between Organization, Users, and Roles tabs,
so that I can manage different aspects of the organization.

**Acceptance Criteria:**
- [ ] Left sidebar shows 3 items: Organization (tree icon), Users (users icon), Roles (shield icon)
- [ ] Active tab is visually highlighted
- [ ] Clicking tab switches content area without full page reload
- [ ] URL updates to reflect active tab: `/admin`, `/admin/users`, `/admin/roles`
- [ ] Browser back/forward navigation works between tabs

**Proto mapping:** Frontend-only — TanStack Router nested routes

## Flows

### Flow: Enter Admin
1. User clicks "Admin" icon in AppSidebar
2. URL changes to `/admin`
3. Admin layout loads with sub-sidebar (Organization, Users, Roles)
4. Organization tab content shows department tree (default)

## States

| Screen | Empty | Loading | Loaded | Error | Permission Denied |
|--------|-------|---------|--------|-------|-------------------|
| Admin layout | n/a | Spinner | Sub-sidebar + content | "Unable to load admin" + retry | Redirect to workspace home |
