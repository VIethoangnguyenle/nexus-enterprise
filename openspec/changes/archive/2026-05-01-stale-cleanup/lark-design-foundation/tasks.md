## 1. Design Tokens Update

- [x] 1.1 Update `index.css` @theme block — replace all color tokens with Lark-accurate values (bg-primary, bg-secondary, bg-tertiary, bg-rail, bg-hover, bg-active, accent, text colors)
- [x] 1.2 Update border radius tokens — sm:4px, md:6px, lg:8px
- [x] 1.3 Remove shadow-glow and shadow-lg tokens, replace with minimal shadows
- [x] 1.4 Update scrollbar CSS — 4px width, transparent by default, visible on container hover

## 2. Button Primitive Restyle

- [x] 2.1 Rewrite `Button.tsx` primary variant — flat bg-accent, no gradient, no glow shadow, no translate-y hover
- [x] 2.2 Update secondary variant — remove glass background, use subtle border only
- [x] 2.3 Update ghost variant — ensure consistent hover:bg-bg-hover pattern
- [x] 2.4 Update danger and success variants — flat tinted backgrounds

## 3. Other Primitives Restyle

- [x] 3.1 Update `Avatar.tsx` — align sizes and border with Lark tokens
- [x] 3.2 Update `Badge.tsx` — use Lark red badge style, smaller sizing
- [x] 3.3 Update `Input.tsx` — dark background, subtle border, no strong focus glow
- [x] 3.4 Update `Select.tsx` — match Input styling
- [x] 3.5 Update `Textarea.tsx` — match Input styling
- [x] 3.6 Update `IconButton.tsx` — flat, no decorative effects

## 4. Workspace Layout Spacing

- [x] 4.1 Update `_workspace.tsx` topbar — reduce height from 52px to 44px, remove backdrop-blur
- [x] 4.2 Tighten ListPanel header padding — py-3 to py-2
- [x] 4.3 Tighten ListPanel item padding — py-2.5 to py-1.5
- [x] 4.4 Update channel header in `channels.$channelId.tsx` — remove bg-tertiary/30 backdrop, tighten padding

## 5. Global Polish

- [x] 5.1 Remove all `backdrop-blur-sm` usage across codebase
- [x] 5.2 Replace any remaining gradient backgrounds in non-button elements
- [x] 5.3 Update Modal.tsx — flat overlay, remove shadow-lg
- [x] 5.4 Update Card.tsx — remove decorative shadows

## 6. Verification

- [x] 6.1 Browser test — login, navigate messaging, verify no visual breakage
- [x] 6.2 Browser test — navigate drive, verify file list renders correctly
- [x] 6.3 Browser test — check all hover states use new token colors
- [x] 6.4 Verify Vite build passes with no warnings
