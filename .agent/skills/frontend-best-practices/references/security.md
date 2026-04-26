# Security — Frontend

## XSS Prevention
- **Never** use `dangerouslySetInnerHTML` — sanitize if absolutely necessary
- User input displayed via JSX `{text}` is auto-escaped by React

## Token Storage
- JWT stored in localStorage via Zustand `persist` middleware
- Injected as `Authorization: Bearer` header via `apiFetch`
- Cleared on logout — `useAuthStore.getState().logout()`
- **Never** expose token in URL params or logs

## Input Validation
- Validate on submit, not on every keystroke
- Server is the source of truth — client validation is UX only
- Disable submit buttons during mutation (`isPending`)

## CORS & Proxy
- Vite proxy in `vite.config.js`: `/api` → `localhost:8080`, `/ws` → `localhost:8081`
- Production: Nginx reverse proxy handles routing
- **Never** call backend directly from browser — always through proxy
