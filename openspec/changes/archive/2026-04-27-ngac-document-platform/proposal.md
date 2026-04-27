## Why

Enterprise document management requires fine-grained, explainable access control that goes beyond simple role-based systems. Organizations need to share documents across company boundaries while maintaining strict isolation by default. NGAC (Next Generation Access Control), as defined by NIST, provides a graph-based policy model where access decisions emerge from structural relationships — not hardcoded rules or role checks.

No existing open-source platform demonstrates NGAC in a fullstack, multi-tenant, deployable context. This system fills that gap.

## What Changes

- **New fullstack platform** built from scratch (Go backend + React frontend + PostgreSQL)
- Implements a complete NGAC policy engine with graph-based access decisions
- Multi-tenant isolation where each company is a Policy Class
- Cross-company document sharing modeled entirely through NGAC graph relationships
- Public document access via NGAC graph (no boolean flags)
- Document workflow (Draft → Approved) enforced through graph mutations
- Time-based editing constraints via a policy constraint layer
- Every access decision returns a human-readable explanation of WHY access was granted or denied
- Dockerized deployment with `docker-compose up`

## Capabilities

### New Capabilities
- `ngac-engine`: Core NGAC policy engine — graph storage, traversal, access decisions, and explanation generation using PostgreSQL adjacency-list model with in-memory graph
- `user-auth`: User authentication with username/password registration, JWT-based sessions, and automatic NGAC node creation for new users
- `document-management`: Document CRUD operations — upload, retrieve, delete with NGAC-enforced access checks on every operation
- `document-workflow`: Draft → Approved lifecycle — documents start as drafts, reviewers approve them, only approved docs can be shared or published
- `cross-company-sharing`: Cross-tenant document sharing via NGAC graph — creates scoped Object Attributes under SharedDocs with targeted associations
- `public-documents`: Public document visibility via PublicDocs OA and PublicUsers UA — all users belong to PublicUsers, published docs assigned to PublicDocs
- `policy-constraints`: Dynamic policy constraint layer — time-based editing restrictions (weekday-only) evaluated after graph traversal, separate from business logic
- `access-explainer`: Access decision explanation API — returns the full traversal path showing exactly why access was granted or denied
- `admin-management`: Company and department administration — create Policy Classes, User Attributes, manage user assignments
- `frontend-ui`: React SPA with login, dashboard, document management, sharing controls, approval workflow, and permission explanation views
- `docker-deployment`: Docker Compose setup with PostgreSQL, Go backend, and React frontend services plus seed data

### Modified Capabilities
_(none — greenfield project)_

## Impact

- **New codebase**: Go backend (`/backend`), React frontend (`/frontend`), Docker config at root
- **Database**: PostgreSQL with NGAC graph tables (ngac_nodes, ngac_assignments, ngac_associations, ngac_prohibitions) plus users and documents tables
- **APIs**: REST endpoints for auth, documents, sharing, access checks, and admin operations
- **Dependencies**: Go (gin/chi, pgx, jwt-go), React (vite, react-router), PostgreSQL 16
- **Deployment**: Single `docker-compose.yml` bringing up all 3 services
