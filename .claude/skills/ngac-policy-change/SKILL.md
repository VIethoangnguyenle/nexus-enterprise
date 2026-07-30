---
name: ngac-policy-change
description: Use when changing anything that affects an NGAC authorization decision — node types, assignments, associations, prohibitions, operations, the PDP decision order, or the shape of the in-memory graph. Covers what to verify and in what order so a permission change cannot silently widen access. Not a process skill; it assumes superpowers already governs planning, TDD, and verification.
---

# Changing the NGAC policy model

Authorization bugs in this system are quiet. A wrong assignment does not crash — it grants
someone access and nothing observes it. There is no audit trail of decisions to catch it after
the fact, so the check has to happen before the change lands.

## Before touching code

1. **Read the capability spec** in `docs/specs/` that owns the behaviour you are changing —
   `tenant-ngac-init`, `batch-access-check`, or whichever applies. If none covers it, the change
   is introducing a capability and needs a new spec, not an edit to an existing one.
2. **Locate the layer.** The four are separate on purpose and a change usually belongs to exactly
   one: PAP writes policy (`pap_*.go`), PIP reads the graph out of Postgres (`pip_*.go`), PDP
   decides (`pdp_*.go`), EPP invalidates caches on change (`epp_*.go`). Changing the decision in
   the PIP layer, or the graph in the PDP layer, is how the layering rots.

## The order that matters

**Write the failing test vector first, and write the deny case first.**
`backend/services/policy/internal/ngac/pdp_vnpay_scenarios_test.go` is the worked example to
follow — it builds a realistic graph and asserts decisions across it.

The deny case comes first because the system's default is already DENY. A test that only proves
your new path returns ALLOW will pass against code that returns ALLOW unconditionally. It is the
deny assertion that pins the boundary. Concretely, for any new path assert all of:

- the intended subject is allowed
- a subject one hop short of the required UA is denied
- a subject reaching the object through a **different PC** is denied — the intersection principle
  is where cross-tenant leaks live
- if a prohibition can apply, that it overrides an otherwise-ALLOW

Only then implement.

## Things that are easy to get wrong here

- **Objects are not in the graph.** `LoadGraph()` loads `U`, `UA`, `OA`, `PC` only. If you find
  yourself adding an `O` node to make a check work, the check is at the wrong level — it belongs
  on the parent OA.
- **Never build a node name or operation string inline.** They come from `backend/ngac`. A raw
  `fmt.Sprintf("PC_%s", id)` compiles fine after someone renames the helper, and quietly stops
  matching anything.
- **Graph writes need EPP invalidation.** Runtime decisions read the in-memory graph and never
  the database. A write that skips the invalidation path leaves every service authorizing against
  a stale graph for as long as the process lives — which in production is indefinitely.
- **Prohibitions only apply to ALLOW.** They are deny-overrides evaluated after BFS, not an
  independent decision path. A prohibition test that starts from a DENY proves nothing.

## Closing the loop

Run the deny cases, then `make test s=policy`, then `make test` for the services that call
`CheckAccess` — approval, asset, and drive are the PEPs.

Update the spec in `docs/specs/` in the same commit. A policy change whose spec still describes
the old behaviour has left the next agent a document that lies, and this codebase has already
been bitten by that.
