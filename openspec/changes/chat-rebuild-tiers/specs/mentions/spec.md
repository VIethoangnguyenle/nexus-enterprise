# @Mention Autocomplete — Spec

## User Stories

### S1: @Mention Autocomplete in Editor
As a workspace member typing a message,
I want to type @ and see a dropdown of channel members,
so that I can mention someone and notify them.

Acceptance Criteria:
- [ ] Typing @ in ChatEditor triggers member autocomplete dropdown
- [ ] Dropdown shows channel members filtered by typed text after @
- [ ] Selecting a member inserts formatted @mention into content
- [ ] @mentions are highlighted in sent messages (already works via MessageContent)
- [ ] Pressing Escape or clicking outside closes dropdown
- [ ] Dropdown shows avatar + display name for each member

Proto mapping: `GET /channels/:id/members` (REST — EXISTS for member list)
Frontend-only: YES — member list API exists, mention extraction exists in ChatEditor

## Flow

### @Mention
1. User types "@" in ChatEditor
2. Autocomplete dropdown appears above cursor with channel members
3. User continues typing to filter (e.g., "@ngu" filters to "Nguyen Van A")
4. User clicks or presses Enter on a member
5. @mention inserted as formatted text
6. On send, ChatEditor extracts mentions (already implemented)

## States
- **Hidden**: No @ typed
- **Open**: Dropdown visible with filtered members
- **No results**: "No members matching"
- **Loading**: Spinner while fetching members (first time)
