# Tasks — Multi-User Group Chat Test

## Phase 1: Backend — User Lookup Endpoint
- [x] **T1.1** Add `GET /api/users/lookup?username=X` to auth REST handler (returns `{ id, username, ngac_node_id }`)
- [x] **T1.2** Build auth service — verify endpoint works

---

## Phase 2: Frontend — Invite & Auth Fixes
- [x] **T2.1** Add `lookupUser(username)` to `api/auth.ts`
- [x] **T2.2** Add `inviteMember(wsId, ngacNodeId)` and `listMembers(wsId)` to `api/workspaces.ts`
- [x] **T2.3** Create `useInviteMember` mutation hook in `hooks/useWorkspaces.ts`
- [x] **T2.4** Create `InviteMemberForm.tsx` component (username input + invite button + feedback)
- [x] **T2.5** Wire invite form into workspace settings or header area
- [x] **T2.6** Fix `currentUserId` in `channels.$channelId.tsx` — read from `useAuthStore`
- [x] **T2.7** Fix ThreadPanel close button — replace `✕` with Lucide `X` icon
- [x] **T2.8** Vite build passes

---

## Phase 3: Multi-User UI Test — Setup
- [ ] **T3.1** `make run` — all services healthy
- [ ] **T3.2** Tab 1: Register "alice" / password123 → Create workspace "Test Team"
- [ ] **T3.3** Tab 1: Create channel "team-chat"
- [ ] **T3.4** Tab 2: Register "bob" / password123
- [ ] **T3.5** Tab 1: Invite "bob" via UI
- [ ] **T3.6** Tab 2: Verify Bob sees "Test Team" workspace → Navigate to "team-chat"
- [ ] **T3.7** Tab 3: Register "carol" / password123
- [ ] **T3.8** Tab 1: Invite "carol" via UI
- [ ] **T3.9** Tab 3: Verify Carol sees workspace → Navigate to "team-chat"

---

## Phase 4: Multi-User UI Test — Conversation
- [ ] **T4.1** Alice sends: "Chào cả nhóm! 👋"
- [ ] **T4.2** Bob sends: "Hello Alice! Mình là Bob"
- [ ] **T4.3** Carol sends: "Hi mọi người, Carol đây!"
- [ ] **T4.4** Verify: 3 messages, 3 distinct avatar colors, correct sender names, chronological order
- [ ] **T4.5** Alice uploads a .txt or .md file via paperclip button
- [ ] **T4.6** Bob uploads an image file (.png/.jpg) via paperclip button
- [ ] **T4.7** Carol sends: "Đã nhận file rồi, cảm ơn!"
- [ ] **T4.8** Verify: FilePreviewCard renders for both files, correct filenames displayed
- [ ] **T4.9** Bob reacts 👍 to Alice's first message
- [ ] **T4.10** Carol reacts ❤️ to Bob's image message
- [ ] **T4.11** Verify: reactions display correctly under messages
- [ ] **T4.12** Alice pins Carol's "Hi mọi người" message
- [ ] **T4.13** Verify: pin indicator appears on message

---

## Phase 5: UX/UI Audit
- [ ] **T5.1** Screenshot each major state, annotate design issues found
- [ ] **T5.2** Check: message grouping visual (same sender consecutive → grouped, different → avatar shown)
- [ ] **T5.3** Check: timestamp display consistency
- [ ] **T5.4** Check: empty state → populated state transition smoothness
- [ ] **T5.5** Check: chat input area — paperclip, emoji, send button alignment & sizing
- [ ] **T5.6** Check: file preview card visual quality in message context
- [ ] **T5.7** Check: hover actions bar positioning and visibility
- [ ] **T5.8** Check: sidebar channel list — active state, unread badges
- [ ] **T5.9** Check: overall color palette, spacing, typography coherence
- [ ] **T5.10** Produce UX audit summary with issues + priority ranking
