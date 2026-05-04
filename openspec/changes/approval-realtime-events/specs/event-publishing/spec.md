# Event Publishing — Approval Service

## User Stories

### S1: Approval service publishes event on request creation
As an **approval service operator**,
I want the service to publish a Kafka event when a new approval request is created,
so that downstream consumers can react in real-time.

**Acceptance Criteria:**
- [ ] When `CreateApprovalRequest` RPC or `POST /requests` succeeds, a JSON event is published to `approval.events` Kafka topic
- [ ] Event contains: `request_id`, `template_name`, `entity_type`, `status` ("pending"), `created_by`, `scope_oa_id`, `timestamp`
- [ ] Event contains `assignee_node_ids` — list of user_node_ids who are assigned at current step
- [ ] If Kafka is unavailable, request creation still succeeds (fire-and-forget, graceful degradation)
- [ ] Event is published AFTER database transaction commits (no event-on-rollback)

Proto mapping: `CreateRequest` RPC (EXISTS) — needs post-commit event hook
Backend work: NEW `events/producer.go` in approval service

---

### S2: Approval service publishes event on approve action
As an **approval service operator**,
I want the service to publish a Kafka event when a request is approved,
so that the requester and other stakeholders see the update in real-time.

**Acceptance Criteria:**
- [ ] When `Approve` RPC or `POST /requests/:id/approve` succeeds, event is published to `approval.events`
- [ ] Event contains: `request_id`, `template_name`, `status` (new status after approval), `actor_node_id`, `created_by` (original requester), `timestamp`
- [ ] If approval completes all steps, `status` = "approved"; if more steps remain, `status` = "pending" (step advanced)
- [ ] Event fire-and-forget: approval succeeds even if Kafka publish fails

Proto mapping: `Approve` RPC (EXISTS)

---

### S3: Approval service publishes event on reject action
As an **approval service operator**,
I want the service to publish a Kafka event when a request is rejected,
so that the requester sees the rejection immediately.

**Acceptance Criteria:**
- [ ] When `Reject` RPC or `POST /requests/:id/reject` succeeds, event is published to `approval.events`
- [ ] Event contains: `request_id`, `template_name`, `status` ("rejected"), `actor_node_id`, `created_by`, `comment`, `timestamp`
- [ ] Event fire-and-forget

Proto mapping: `Reject` RPC (EXISTS)

---

### S4: Approval service publishes event on batch approve
As an **approval service operator**,
I want batch approvals to publish one event per successfully approved request,
so that each request's stakeholders are notified individually.

**Acceptance Criteria:**
- [ ] `BatchApprove` publishes N events (one per approved request in `approved_ids`)
- [ ] Failed items in `failed_ids` do NOT generate events
- [ ] Same event schema as S2

Proto mapping: `BatchApprove` RPC (EXISTS)

---

## Event Schema

```json
{
  "request_id": "uuid",
  "template_name": "Quick Approval",
  "entity_type": "purchase",
  "status": "pending|approved|rejected",
  "action": "created|approved|rejected|step_advanced",
  "actor_node_id": "user-node-id",
  "created_by": "user-node-id-of-requester",
  "assignee_node_ids": ["user-node-id-1", "user-node-id-2"],
  "scope_oa_id": "default",
  "comment": "",
  "timestamp": 1714600000
}
```

## Flow

```
Flow: Event Publishing
1. REST handler calls domain.Service method
2. Domain method executes business logic + DB transaction
3. On success, handler calls producer.PublishApprovalEvent(event)
4. Producer serializes to JSON, sends to "approval.events" topic async
5. If Kafka unavailable → log warning, continue
```
