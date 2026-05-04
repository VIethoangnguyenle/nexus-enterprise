# Policy Code Cleanup — Hard-code Elimination, Nesting Reduction, LRU Optimization

## Status: PROPOSED
## Created: 2026-05-04
## Author: AI Code Review

---

## Problem

Code review of `backend/services/policy/internal/ngac/` reveals 17 findings across 5 categories:

1. **Hard-coded string literals** scattered across 7+ files (`"ALLOW"`, `"DENY"`, `"UA"`, `"U"`, cache key prefixes, scope strings) with no compile-time protection
2. **Deep nesting** in `Decide()` reaching 6 levels (if→if→if→if→goroutine→if)
3. **O(N) LRU promote** on the hot path — every shard cache hit triggers a linear scan of up to 1000 entries
4. **Duplicated patterns** — cache key prefixes, scope format strings, Redis key deletion loops
5. **Dead code** — deprecated `GetUserByNGACNodeID` crossing service boundaries

## Scope

**In scope:**
- Extract typed constants for decision outcomes, node types usage, cache key prefixes, scope strings
- Refactor `Decide()` nesting via method extraction
- Replace slice-based LRU with `container/list` for O(1) promote/evict
- Remove deprecated dead code
- DRY up duplicated loop patterns in cache invalidator

**Out of scope:**
- No API/proto changes
- No behavioral changes — all optimizations are internal
- No `BatchCheckAccess` parallelization (separate change)
- No cache TTL configurability (infrastructure concern, separate change)

## Evidence

All findings are from direct code reading with line-level references. See full review at `.gemini/antigravity/brain/840ae440-c788-4403-b850-666572cc2e1c/policy_code_review.md`.

## Impact

- **Performance**: LRU fix eliminates O(N) on every access check hot path → O(1)
- **Correctness**: Constants prevent typo-induced bugs (currently `"ALOW"` would silently fail)
- **Maintainability**: Reduced nesting, DRY patterns, cleaner code
- **Risk**: LOW — all changes are internal refactors, no API surface changes

## Success Criteria

1. All tests pass (`go test ./services/policy/...`)
2. Zero new hard-coded decision/node-type strings in `ngac/` package
3. `Decide()` max nesting ≤ 3 levels
4. LRU promote is O(1) via linked list
5. No behavioral changes — identical access decisions before/after
