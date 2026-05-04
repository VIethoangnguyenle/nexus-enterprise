# Search — Spec

## User Stories

### S1: Search Messages in Channel
As a workspace member,
I want to search for messages within a channel,
so that I can find specific conversations or information.

Acceptance Criteria:
- [ ] ChannelInfoPanel "Search" tab has a text input field
- [ ] Typing a query and pressing Enter/debounce triggers search
- [ ] Results show matching messages with sender, timestamp, and highlighted content
- [ ] Clicking a search result navigates/scrolls to that message in the chat
- [ ] Empty results: "No messages found" state
- [ ] Minimum 2 characters before search triggers
- [ ] Loading spinner during search

Proto mapping: `GET /channels/:id/search?q=&limit=` (REST — EXISTS)
Frontend-only: YES

## Flow

### Search
1. User opens ChannelInfoPanel → "Search" tab
2. User types search query in input
3. System calls `GET /channels/:id/search?q=:query&limit=20`
4. Results render below input
5. User clicks a result → message highlighted in chat view

## States
- **Initial**: Search input with placeholder "Search messages..."
- **Loading**: Spinner below input
- **Results**: List of matching messages
- **Empty**: "No messages found for ':query'"
- **Error**: Retry button
