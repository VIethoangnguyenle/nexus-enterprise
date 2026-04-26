## 1. Project Setup & Infrastructure

- [x] 1.1 Create project directory structure: `/backend` (Go), `/frontend` (React), root `docker-compose.yml`
- [x] 1.2 Initialize Go module in `/backend` with dependencies (chi, pgx, jwt-go, bcrypt, uuid)
- [x] 1.3 Initialize React+Vite project in `/frontend` with dependencies (react-router, zustand, axios)
- [x] 1.4 Create `docker-compose.yml` with PostgreSQL, backend, and frontend services
- [x] 1.5 Create PostgreSQL init script with NGAC schema (ngac_nodes, ngac_assignments, ngac_associations, ngac_prohibitions, users, documents)

## 2. NGAC Engine Core

- [x] 2.1 Implement NGAC node types and in-memory graph data structures (Go)
- [x] 2.2 Implement PostgreSQL store layer: CRUD for nodes, assignments, associations
- [x] 2.3 Implement graph loading from PostgreSQL on startup
- [x] 2.4 Implement assignment validation (type compatibility, cycle detection)
- [x] 2.5 Implement access decision function: user→UA traversal, object→OA traversal, association matching, PC intersection check
- [x] 2.6 Implement access explanation builder: return traversal path for ALLOW, missing path details for DENY

## 3. Policy Constraints Layer

- [x] 3.1 Implement constraint framework: registerable constraints with name, target operations, condition function, effect
- [x] 3.2 Implement weekday-only editing constraint (deny write/upload on Saturday/Sunday)
- [x] 3.3 Integrate constraint evaluation into access decision flow (post-graph-traversal)

## 4. Authentication

- [x] 4.1 Implement user registration endpoint: create user record + NGAC User node + assign to department UA + assign to PublicUsers UA
- [x] 4.2 Implement login endpoint with bcrypt password verification and JWT generation
- [x] 4.3 Implement JWT middleware for protected routes

## 5. Document Management API

- [x] 5.1 Implement document upload endpoint: save file, create NGAC Object node, assign to DraftDocs OA
- [x] 5.2 Implement document listing endpoint with NGAC access filtering
- [x] 5.3 Implement document retrieval endpoint with NGAC access check
- [x] 5.4 Implement document deletion endpoint with NGAC access check and graph cleanup

## 6. Document Workflow

- [x] 6.1 Implement approval endpoint: verify approve access, move document OA from DraftDocs to ApprovedDocs
- [x] 6.2 Implement precondition checks: sharing/publishing require document in ApprovedDocs

## 7. Cross-Company Sharing

- [x] 7.1 Implement share endpoint: create scoped OA under SharedDocs, assign document, create association to target UA
- [x] 7.2 Implement revoke sharing endpoint: remove scoped OA, assignments, and association
- [x] 7.3 Implement list shares endpoint: return active shares for a document

## 8. Public Documents

- [x] 8.1 Implement publish endpoint: assign document to PublicDocs OA
- [x] 8.2 Implement unpublish endpoint: remove assignment from PublicDocs OA

## 9. Admin & Access Explanation API

- [x] 9.1 Implement companies listing endpoint (Policy Classes)
- [x] 9.2 Implement departments listing endpoint (UAs under a PC)
- [x] 9.3 Implement users listing endpoint with UA memberships
- [x] 9.4 Implement POST /api/access/check endpoint with full explanation response

## 10. Seed Data

- [x] 10.1 Create seed function: PC_Acme, PC_Beta, PC_Global, department UAs, DraftDocs/ApprovedDocs/SharedDocs/PublicDocs OAs, PublicUsers UA
- [x] 10.2 Create seed users: alice (Acme_Finance), bob (Acme_HR), charlie (Beta_Engineering), dave (Beta_Marketing) — all also in PublicUsers
- [x] 10.3 Create seed documents and NGAC associations for demo scenarios
- [x] 10.4 Create seed data for cross-company sharing demo (one shared doc, one public doc)

## 11. Frontend — Auth & Layout

- [x] 11.1 Create app layout with navigation bar, auth context (Zustand), and React Router setup
- [x] 11.2 Create login page with form and API integration
- [x] 11.3 Create registration page with company/department selection dropdowns

## 12. Frontend — Document Dashboard

- [x] 12.1 Create document dashboard with tabs: My Documents, Shared With Me, Public Documents
- [x] 12.2 Create document card component showing title, status (draft/approved), owner, actions
- [x] 12.3 Create document upload form with file picker and title input

## 13. Frontend — Sharing & Workflow

- [x] 13.1 Create sharing dialog: company/department selector, operation checkboxes, confirm action
- [x] 13.2 Create approval button on draft documents with confirmation
- [x] 13.3 Create public toggle (Make Public / Make Private) on approved documents
- [x] 13.4 Create access explanation modal showing NGAC traversal path

## 14. Frontend — Polish & Styling

- [x] 14.1 Apply premium design system: dark theme, gradients, glassmorphism, modern typography
- [x] 14.2 Add micro-animations: hover effects, transitions, loading states
- [x] 14.3 Add responsive layout refinements and error handling

## 15. Docker & Deployment

- [x] 15.1 Create backend Dockerfile (multi-stage: Go build → scratch/alpine)
- [x] 15.2 Create frontend Dockerfile (multi-stage: Vite build → nginx)
- [x] 15.3 Configure nginx to proxy API requests to backend
- [x] 15.4 Test full `docker-compose up` end-to-end

## 16. Verification

- [x] 16.1 Test: cross-company access denied by default
- [x] 16.2 Test: shared document accessible by target company
- [x] 16.3 Test: public document accessible by all users
- [x] 16.4 Test: draft document cannot be shared
- [x] 16.5 Test: editing blocked on weekends (simulate via constraint)
- [x] 16.6 Test: approval required before visibility
