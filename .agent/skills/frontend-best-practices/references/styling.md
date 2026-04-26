# CSS & Styling

## Design System
- Vanilla CSS only — no Tailwind, no CSS-in-JS
- Design tokens via CSS custom properties in `index.css`
- BEM-like naming: `sidebar-item`, `sidebar-item-icon`, `auth-card`

## Rules
- Use existing CSS classes from `index.css` — don't create ad-hoc inline styles
- Responsive: mobile-first with `@media (min-width: ...)`
- Animations: use `transition` and `@keyframes` for micro-interactions
- Dark mode: already default — maintain dark-first palette
- `className` conditional: `\`card \${active ? 'active' : ''}\``
