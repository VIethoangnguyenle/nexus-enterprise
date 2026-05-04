# Approval Re-Architecture

## Revised Direction (User Checkpoint)

**Approach**: CONTROLLED REVALIDATION + SELECTIVE REWRITE
- NOT full teardown
- NOT "fix bugs on existing code"  
- Validate every layer through EXECUTION, then: works → KEEP, broken/unclear → REWRITE
- "Clean code" ≠ "correct code" — only verified behavior is trusted

## Evidence Summary

- **Backend service**: EXISTS — clean architecture layers. **NOT YET VERIFIED through execution.**
- **Proto**: EXISTS — well-defined. **Contract assumed correct (216 lines, all RPCs).**
- **DB**: EXISTS — tenant-isolated schema. **NOT YET VERIFIED: constraints, data flow.**
- **Frontend**: EXISTS — 10 components, hooks, API layer. **NOT YET VERIFIED against real backend.**

## Product Assessment

- **Size**: M — Validation + selective rewrite of broken layers
- **Risk**: Medium — Integration risk, tenant provisioning unknown, NGAC enforcement unverified
- **Target user**: Workspace admin (templates) + workspace member (submits/approves)
- **Core action**: Admin creates template → Member submits request → Approver acts → Request completes

## Execution Plan

### Phase A: End-to-End Validation (BẮT BUỘC FIRST)
1. Boot approval service
2. Provision tenant schema
3. Run full flow: create template → create request → approve → verify completion
4. Document every failure

### Phase B: Layer-by-Layer Verdict
For each layer (backend domain, store, REST, frontend):
- KEEP if verified working
- REWRITE if broken or unverifiable
- Decision documented with evidence

### Phase C: Selective Rewrite
Only rewrite layers that failed validation

### Phase D: Re-verify
Full end-to-end test after rewrites

## Scope

### In scope
- Full end-to-end validation of existing system
- Selective rewrite of broken/unclear layers
- Frontend fixes for UI-backend integration
- NGAC permission verification

### Out of scope
- Full teardown / greenfield rebuild
- New features beyond current proto contract
- Notification system, analytics
- Multi-tenant provisioning UI

## Success Criteria
1. Service boots and responds to health check
2. Tenant provisioning creates schema successfully
3. Template CRUD operations verified through REST
4. Full request lifecycle verified: create → match → assign → approve → complete
5. Frontend displays real data for all 5 tabs
6. Permission boundaries enforced (NGAC)
