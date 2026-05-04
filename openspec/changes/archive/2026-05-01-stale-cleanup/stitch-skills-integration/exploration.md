# Exploration: Stitch Skills Integration

## Context
User yêu cầu tích hợp bộ skills từ `google-labs-code/stitch-skills` GitHub repository vào dự án NGAC, sử dụng chúng như rule bắt buộc khi làm việc với Stitch UI.

## Research Findings

### Repository Structure
`stitch-skills` cung cấp 8 skills:
1. **stitch-design** — Unified entry point: prompt enhancement + design system + screen generation/editing
2. **react-components** — Converts Stitch HTML → modular React components via AST validation
3. **enhance-prompt** — Transforms vague UI ideas → Stitch-optimized prompts
4. **design-md** — Analyzes Stitch projects → synthesizes semantic DESIGN.md
5. **taste-design** — Premium anti-generic design standards (anti-slop enforcement)
6. **stitch-loop** — Autonomous iterative build loop with baton-passing
7. **shadcn-ui** — shadcn/ui integration (not relevant for NGAC)
8. **remotion** — Video generation from Stitch screens (not relevant for NGAC)

### What Was Installed
6 of 8 skills installed to `.agent/skills/`:
- `stitch-design`, `stitch-react-components`, `stitch-enhance-prompt`
- `stitch-design-md`, `stitch-taste-design`, `stitch-loop`

Skipped: `shadcn-ui` (not used), `remotion` (not needed)

### Infrastructure Created
1. **`.stitch/metadata.json`** — Complete screen registry with 24 screens mapped by slug
2. **`.stitch/DESIGN.md`** — Semantic design system with all tokens, colors, typography, components
3. **`.stitch/WORKFLOW.md`** — Mandatory 6-step protocol for all UI tasks
4. **`.stitch/scripts/fetch-stitch.sh`** — High-reliability curl wrapper for downloading Stitch assets
5. **`.stitch/designs/`** — Staging directory for downloaded HTML/PNG assets

## Key Process Changes

### Before (Legacy)
1. Look at Stitch screenshot
2. Guess colors, spacing, layout
3. Write arbitrary CSS → visual drift
4. No verification against source

### After (Stitch-First)
1. **Fetch** raw HTML via `mcp_stitch_get_screen`
2. **Download** HTML → `.stitch/designs/{page}.html`
3. **Extract** exact tokens, grid templates, class combinations
4. **Cross-reference** against `.stitch/DESIGN.md`
5. **Implement** using EXACT values from source
6. **Verify** at 375/768/1280px against screenshot

## Integration Points

### Skill Usage Triggers
| Trigger | Skill |
|---------|-------|
| Any UI task | `stitch-design` → read WORKFLOW.md |
| Need new Stitch screen | `stitch-design` → text-to-design workflow |
| Editing existing screen | `stitch-design` → edit-design workflow |
| Converting HTML to React | `stitch-react-components` |
| Vague prompt from user | `stitch-enhance-prompt` |
| Updating design system | `stitch-design-md` |
| Quality standard check | `stitch-taste-design` anti-patterns |

## Status: ✅ COMPLETE
All skills installed, infrastructure created, workflow documented.
