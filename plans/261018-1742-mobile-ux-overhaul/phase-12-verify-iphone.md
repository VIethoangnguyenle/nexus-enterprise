---
phase: 12
title: "Verify on a real iPhone"
status: pending
priority: P1
effort: "0.5d"
dependencies: [2, 3, 5, 6, 7, 8, 9, 10, 11, 13]
---

# Phase 12: Verify on a real iPhone

## Overview
Prove the whole overhaul on the device it is for: an installed PWA on iPhone, both workspace kinds, light and dark.

## Requirements
- Functional: every mockup § walked on device; touch-only behaviours (edge back, row swipe, long-press slide,
  double-tap, keyboard drag, pinch) verified by a person (Zane) since headless browsers cannot.
- Non-functional: 390px e2e screenshot suite committed under `plans/261018-1742-mobile-ux-overhaul/reports/`;
  `make build-check`, `make test`, `make check-layering`, `make check-ngac`, frontend test/lint/build green.

## Architecture
Deploy to a preview (or production after approval) per `docs/deployment.md`; checklist derived from mockup §1–§10.

## Related Code Files
- Create: `plans/261018-1742-mobile-ux-overhaul/reports/iphone-checklist.md`, e2e screenshots

## Implementation Steps
1. Generate the checklist from the specs' acceptance lines.
2. Run the e2e suite at 390px (both kinds, both themes).
3. Zane runs the device pass; failures go back to the owning phase via `ak:debug` → `ak:fix`.
4. Update specs touched; `ak:code-review` over the branch.

## Success Criteria
- [ ] Checklist fully ticked on device
- [ ] All gates green; code review findings resolved

## Risk Assessment
iOS PWA quirks (safe areas, keyboard, standalone back) found late → phases 02, 09, 10 each do a quick device check
when a deploy preview exists, not only here.
