# WS Poll Vote Cache Injection

## Proto Mapping
- WS Event: `PollVoteEvent` — exists in ws.proto (field 11)
- Fields: `poll_id`, `channel_id`, `option_id`, `vote_count`, `total_votes`

## User Stories

### Story 1: Real-time poll vote updates
As a user viewing a poll, I want to see vote counts update live when others vote.

**Acceptance Criteria:**
- [ ] When another user votes, the option's `vote_count` and poll's `total_votes` update
- [ ] No HTTP GET `/polls/{id}` triggered by WS event
- [ ] Vote percentages recalculate in real-time

### Story 2: Optimistic vote
As a user, I want my vote to appear instantly when I select an option.

**Acceptance Criteria:**
- [ ] Clicking a poll option immediately increments `vote_count` and `total_votes`
- [ ] If API fails, vote count rolls back
- [ ] WS event deduplicates with optimistic update

**States:**
- Normal: Vote counts shown
- Loading: Optimistic count increment
- Error: Rollback vote count

## Flows

### Flow: WS poll vote event
1. Receive `pollVoteEvent`
2. Update `['poll', pollId]` cache → find option, set `vote_count` and `total_votes`
3. No HTTP call
