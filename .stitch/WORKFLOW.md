# Stitch-First UI Development Workflow — MANDATORY

> **This is a NON-NEGOTIABLE rule.** Every UI task MUST follow this workflow.
> Violation = Visual debt. Visual debt = Rejected.

## Core Principle

**"Source First, Code Second."** Never write UI code from memory, estimation, or screenshot inspection. Always start from the raw Stitch HTML source as the single source of truth.

---

## The 6-Step Protocol

### Step 1: Identify the Screen
- Consult `.stitch/metadata.json` for the Screen ID
- If the screen doesn't exist in the registry, use `mcp_stitch_list_screens` to find it
- Map the task to one or more Stitch screens

### Step 2: Fetch Raw Source
```
mcp_stitch_get_screen(
  projectId: "14852434379132121789",
  screenId: "{screen_id}",
  name: "projects/14852434379132121789/screens/{screen_id}"
)
```
- Download `htmlCode.downloadUrl` → `.stitch/designs/{page}.html`
- Download `screenshot.downloadUrl` → `.stitch/designs/{page}.png`
- Use `.stitch/scripts/fetch-stitch.sh` for high-reliability download

### Step 3: Extract Tokens & Layout
From the raw HTML, extract:
1. **CSS Grid template** — exact `grid-template-columns` values
2. **Color tokens** — exact Tailwind classes or hex codes
3. **Spacing values** — padding, margin, gap in px
4. **Typography** — font-size, font-weight, letter-spacing, line-height
5. **Component patterns** — exact class combinations for badges, buttons, cards
6. **Layout structure** — DOM hierarchy, responsive breakpoints

### Step 4: Cross-Reference DESIGN.md
- Verify extracted tokens against `.stitch/DESIGN.md`
- If new tokens are found, update DESIGN.md
- Ensure semantic color names match functional roles

### Step 5: Implement
- Write React components using EXACT tokens from Step 3
- No arbitrary CSS values — every value must trace back to source
- Follow existing project architecture (TanStack Router + TanStack Query + Zustand)
- Apply responsive rules from DESIGN.md Section 5

### Step 6: Verify
- Browser-test at 375px (mobile), 768px (tablet), 1280px (desktop)
- Compare rendered output against Stitch screenshot
- Confirm all tokens match source HTML

---

## Quick Reference — Stitch MCP Tools

| Action | Tool | Key Parameters |
|--------|------|----------------|
| List all screens | `mcp_stitch_list_screens` | `projectId: "14852434379132121789"` |
| Get screen detail | `mcp_stitch_get_screen` | `projectId`, `screenId`, `name` |
| Generate new screen | `mcp_stitch_generate_screen_from_text` | `projectId`, `prompt`, `deviceType` |
| Edit existing screen | `mcp_stitch_edit_screens` | `projectId`, `selectedScreenIds[]`, `prompt` |
| List projects | `mcp_stitch_list_projects` | — |
| Get project info | `mcp_stitch_get_project` | `name: "projects/{id}"` |

---

## Screen Registry (Nexus Hub)

See `.stitch/metadata.json` for the complete screen → ID mapping. Key screens:

| Module | Screen Key | Screen ID |
|--------|-----------|-----------|
| Contacts | `contacts-directory` | `426a5e2c0c384d7796849e1c34c66a16` |
| Contacts | `contacts-profile-popup` | `338dbbb254ce4f73971623afd03fb7e6` |
| Chat | `nexus-chat` | `9fffe37618be439780ff8c84c6f756bf` |
| Chat | `engineering-dept` | `26f33990e1ab4ad6b88f28dea55cf859` |
| Chat (Tablet) | `chat-tablet` | `6185080da8834dfab4607b559587084c` |
| Chat (Mobile) | `chat-list-mobile` | `4b6ae2505ad04d8eaa96b8a800f6c3e4` |
| Drive | `drive-my-files` | `20c61af21ab745be86be38a3c04bc245` |
| Drive | `drive-shared` | `a98bcb54ea154ee8bd33154a6e86e3d1` |
| Approvals | `approvals` | `55cbfca1cea54f968a25ac1548e16792` |
| Approvals | `approval-detail` | `7546030b1c364ca1aca39aed45dad60a` |
| Workplace | `workplace` | `0fe853a3e6394fe5aff3a937420b5c00` |
| Auth | `login` | `c2ac854b2a1c498b8dc882c74ddbdb0f` |
| Workspace | `workspace-selection` | `0a4e176dfc7f4046b97d9aa9bad9ded0` |

---

## Agent Skills Reference

| Skill | Location | Purpose |
|-------|----------|---------|
| `stitch-design` | `.agent/skills/stitch-design/` | Prompt enhancement + design system + screen generation |
| `stitch-react-components` | `.agent/skills/stitch-react-components/` | HTML → React component conversion |
| `stitch-enhance-prompt` | `.agent/skills/stitch-enhance-prompt/` | Prompt polishing for better Stitch results |
| `stitch-design-md` | `.agent/skills/stitch-design-md/` | Synthesize DESIGN.md from project analysis |
| `stitch-taste-design` | `.agent/skills/stitch-taste-design/` | Premium anti-generic design standards |
| `stitch-loop` | `.agent/skills/stitch-loop/` | Iterative autonomous build loop |

---

## Anti-Patterns (BANNED)

| ❌ Pattern | ✅ Required |
|-----------|-------------|
| Coding UI from memory | Fetch Stitch source first |
| Guessing colors/spacing | Extract exact values from HTML |
| Using arbitrary Tailwind classes | Use DESIGN.md tokens |
| Skipping responsive test | Test 375/768/1280px |
| Hardcoded hex in components | Use semantic token names |
| Estimating layout from screenshots | Parse raw HTML grid templates |
