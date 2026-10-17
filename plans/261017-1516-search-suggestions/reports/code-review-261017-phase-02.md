# Code Review: phase 02 (search suggestions foundation)

## Scope
- Files: `frontend/src/lib/recent-searches.ts` (+test), `frontend/src/hooks/useSuggestionPanel.ts`,
  `frontend/src/components/patterns/SearchSuggestions.tsx` (+test), `frontend/src/components/spaces/ChatSearchScreen.tsx` (+test),
  `frontend/src/stores/auth.store.test.ts`. Callers checked: `auth.store.ts:96`, `useModalFocus.ts`, `SearchField.tsx`, `index.css` tokens.
- Focus: uncommitted diff on `main`.
- Wiring into module screens is out of scope (phases 03-05), so it is not reviewed here.

## Overall
The phase meets its acceptance criteria. Call sites are migrated, and the only remaining use of the old key is the
migration branch plus the sign-out test fixture. The combobox state and portal cleanup are sound. I found one real
data-loss path in the legacy migration, and I reproduced it.

## High

**H1. A corrupt legacy Chat key permanently disables Chat history and overwrites the new list.**
- Where: `recent-searches.ts:28`. `parse(raw)` runs outside the `try` in `moveLegacyChat`. The exception reaches
  `readRecent`'s catch (`:44`), so the function returns `[]` before it ever reads the new key. The legacy key is never
  removed, so this repeats on every read.
- Effect: `addRecent` (`:61`) then reads `[]` and writes `[q]`, which overwrites the existing new-key list.
- Reproduced with a vitest probe. The new key held `['a1','b2','c3']` and the legacy key held `'{corrupt'`:
  - `readRecent` returned `[]`.
  - `addRecent('zz')` stored `["zz"]`.
  - The next `readRecent` returned `[]` again, and the legacy key was still present.
- Before this change, a corrupt value only gave `[]` and fixed itself on the next add. Now Chat recents stay broken until
  sign-out. Likelihood is low, but this is exactly the failure the phase's Risk section asks us to guard against.
- Fix: catch the legacy parse inside `moveLegacyChat` and treat an unreadable legacy value as nothing to migrate. Remove
  it, and leave the new key alone:
  ```ts
  let list: string[]
  try { list = parse(raw) } catch { list = [] }
  ```
  Add a test that combines a corrupt legacy key with an existing new key.

## Medium

**M1. The ✕ button is an interactive control nested inside `role="option"`.**
- Where: `SearchSuggestions.tsx:133-147`. Option children are presentational in ARIA, and axe `nested-interactive` flags
  this even with `tabIndex={-1}`.
- Screen-reader effect: the option's name becomes "title + Xoá “title” khỏi Tìm gần đây", read as one string.
- On a phone with VoiceOver or TalkBack there is no Delete key, so removal depends on a button that the assistive tech
  flattens.
- Options:
  - (a) Mark the ✕ `aria-hidden` (pointer use only) and expose removal through Delete, plus a `aria-description`/hint.
    The desktop footer `:80-84` does not mention Delete, which DESIGN.md line 732 requires.
  - (b) Use the APG grid-combobox pattern.
- Either way, add the Delete hint to the footer.

**M2. The phone `screen` variant's lifetime is tied to input focus.**
- Where: `useSuggestionPanel.ts:72,130`. `onBlur` sets `focused=false`, which hides the inline screen.
- DESIGN.md line 713-715: on a phone, the suggestions cover the list until Quay lại/back. iOS "Done" (and any other
  blur) removes them, and the list underneath reappears without the user pressing Back.
- This may be intended to be handled at wiring time, but the hook currently gives the phone screen no way to stay open.
  Decide this before phase 03, for example with a `variant==='screen'` override or an `open` controlled by the caller.

**M3. The popover stacks below sticky layers and is never repositioned for layout shifts.**
- Where: `SearchSuggestions.tsx:98` and `:25-34`.
- Stacking: the portal popover uses `z-dropdown` (10). `#root` has no `isolation`, so `z-sticky` (20) elements in the app
  compete with it at the same level and paint above it. Examples: `SidePanel.tsx:51,55` (md/lg side panel) and
  `DriveScreen.tsx:457` (FAB).
- Repositioning: only `resize` and `scroll` trigger placement. A sidebar collapse, data loading above the box, or a
  font swap leaves the popover where it was.
- Other placement gaps:
  - Nothing flips the popover above the box or limits its height to the space below.
  - `width` can drop below the anchor width near the right edge (`:21`).
- Suggested fixes:
  - Use a layer at or above `z-sticky` for an anchored overlay.
  - Add a `ResizeObserver` on the anchor.
  - Clamp `maxHeight` to `innerHeight - top - EDGE`.

## Low

- **L1.** `useSuggestionPanel.ts:76`: `current` is clamped by index only. When the items change under a marked row, the
  same index now points to a different item. This happens after Delete, or when a loading group earlier in the order
  fills in. Enter then opens something the user never marked. Consider remembering the marked item by `key` instead of
  by index.
- **L2.** `SearchSuggestions.tsx:22`: `setPos` always builds a new object. The capture `scroll` listener also fires when
  the popover's own list scrolls, so every scroll frame re-renders. Skip the update when nothing changed.
- **L3.** `SearchSuggestions.tsx:66`: groups use `key={g.label}`, which collides if two groups share a label. Use the
  index or an explicit key.
- **L4.** `useSuggestionPanel.ts:122`: `aria-controls` points to an id that does not exist while the list is closed, and
  on the phone empty state (`:56-60`, no `id`). axe tolerates this when `aria-expanded=false`. Low risk.
- **L5.** Both keys present: the legacy entries are dropped without being merged (`:30`). This is documented as
  intended. The only realistic source is an old-bundle tab still writing the legacy key during the deploy window, so it
  is acceptable.
- **L6. Scope.** The working tree also contains `e2e/wave1-two-people.mjs` (−183/+116), plus the untracked
  `e2e/_hdr-probe.mjs`, `e2e/chat-wiring-smoke.mjs` and `tsconfig.tsbuildinfo`. None of these belong to phase 02. Keep
  them out of the phase commit. `_hdr-probe.mjs` and `tsbuildinfo` look like scratch/build artifacts.
- **L7.** The plan lists a separate hook test (`phase-02-foundation.md` "Create: useSuggestionPanel.ts + test"). The hook
  is only exercised through the component harness. That is acceptable as coverage, but it differs from the plan.

## Checked, no issue found
- (b) Regressions:
  - No caller reads `ngac-chat-recent-searches` except the migration branch and the sign-out test fixture.
  - `ChatSearchScreen` passes `'chat'` at all 3 call sites.
  - `clearAllRecent` collects keys before removing them (correct iteration) and covers both prefixes.
  - `auth.store.ts:96` is unchanged.
- (c) Migration with storage blocked or a failed `setItem`: the legacy key is kept and the legacy list is returned. This
  is tested in `recent-searches.test.ts:106`.
- (e) Hook mechanics:
  - Setting state during render on a value change is the React-sanctioned pattern and is StrictMode-safe.
  - The portal unmounts through the early return, and the effect cleanup removes both listeners.
  - `useLayoutEffect` places the popover before paint.
  - Esc: React's `stopPropagation` stops the native event before it reaches `useModalFocus`'s `document` keydown, so a
    dialog behind the box stays open.
  - `#root` being `inert` under a modal does not affect the portal in `body`.
- (f) Tokens:
  - `bg-overlay`, `shadow-overlay`, `rounded-overlay`, `bg-accent-wash`, `bg-hover`, `bg-sunk`, `duration-quick` and
    `z-dropdown` all exist in `index.css`.
  - The arbitrary `text-[0.9375rem]` style sizes match 12 existing usages.
  - Primitives are used (`IconButton`, `SearchField`). Rows are `div role=option`, which follows the combobox pattern;
    they are not raw controls.

## Untested (would add)
1. A corrupt legacy key combined with an existing new key (H1).
2. Esc while the box sits inside a `Dialog`: the list closes and the dialog stays open. The `stopPropagation` path is
   the main reason that code exists, and nothing covers it.
3. A Delete on a marked row that has no `onRemove` does nothing. ↑ from no selection wraps to the last row.
4. The marked row after the items change (L1).
5. A touch sequence (`pointerdown`/`touchstart` → `mousedown` → `click`) on a row keeps focus and selects. Today only the
   `user.click` mouse path is covered.
6. Popover placement on scroll and resize.
7. `removeItem` throwing after a successful `setItem` (both keys stay; the next read prefers the new key).

## Recommended actions
1. Fix H1 and add its test. This blocks shipping the migration.
2. Decide M2 (phone lifetime) before phase 03 wires the phone screens.
3. Fix M1 (a11y of ✕ and the Delete hint) and M3 (layer, ResizeObserver, max-height) in this phase or record them for
   phase 03.
4. Keep the e2e and scratch files out of the commit.

## Unresolved questions
- Should the phone `screen` survive a blur (DESIGN.md says it stays until Back)?
- Is a pointer-only ✕ acceptable for phone screen-reader users, or does removal need an accessible path there?
