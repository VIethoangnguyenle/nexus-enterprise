## Context

This is a greenfield multi-tenant document management platform demonstrating NIST NGAC (Next Generation Access Control). The system replaces traditional RBAC with a graph-based policy model where access decisions emerge from structural relationships between users, attributes, and policy classes.

No existing codebase — building from scratch with Go backend, React frontend, PostgreSQL storage, Docker deployment.

**Key constraint:** ALL access control must flow through the NGAC graph. No boolean flags, no role checks, no ad-hoc permission tables.

## Goals / Non-Goals

**Goals:**
- Implement a correct, complete NGAC policy engine with graph traversal and explanation
- Demonstrate multi-tenant isolation via Policy Classes
- Enable cross-company sharing and public documents purely through NGAC graph mutations
- Support document workflow (Draft → Approved) as graph state transitions
- Provide time-based policy constraints as a separate evaluation layer
- Deliver a polished React UI for all document and sharing operations
- Single-command deployment via `docker-compose up`

**Non-Goals:**
- Production-grade file storage (local filesystem is sufficient for MVP)
- OAuth/SSO integration (simple username/password only)
- Real-time collaboration or notifications
- Full NGAC prohibition/obligation spec (implement core subset)
- Horizontal scaling or clustering
- Mobile-responsive UI (desktop-first)

## Decisions

### D1: PostgreSQL Adjacency-List vs Neo4j for Graph Storage

**Choice: PostgreSQL with adjacency-list tables**

- NGAC graphs are small (hundreds to low thousands of nodes), not millions
- Recursive CTEs handle traversal depth of 3-5 levels efficiently
- Single database simplifies deployment and operations
- PostgreSQL's JSONB for node properties provides flexibility
- Neo4j Community Edition has licensing limitations

**Alternatives considered:**
- Neo4j: Natural graph queries via Cypher, but adds operational complexity and another service to Docker stack
- SQLite: Even simpler but lacks concurrent write support needed for multi-user

### D2: In-Memory Graph with Write-Through Persistence

**Choice: Load full NGAC graph into memory on startup, write-through to PostgreSQL on mutations**

- Graph traversal with per-hop DB queries would be 10-100x slower
- The graph fits easily in memory (a few MB even at scale)
- Write-through ensures durability without sacrificing read performance
- On restart, graph is rebuilt from PostgreSQL

**Risk:** Memory graph and DB can diverge if process crashes mid-write → mitigated by rebuild-from-DB on startup

### D3: Go with Chi Router for Backend

**Choice: Go with chi HTTP router**

- Graph traversal is CPU-bound — Go excels here
- Chi is lightweight, stdlib-compatible, production-proven
- Small Docker images (~15MB with scratch base)
- Strong typing catches policy logic bugs at compile time

**Alternatives considered:**
- Gin: Slightly more features but heavier; chi is sufficient
- Node.js/Express: Adequate but weaker for CPU-bound graph operations

### D4: Constraint Layer Separate from Graph

**Choice: Policy constraints (time-based, workflow) evaluated as a post-graph-traversal layer**

- Graph traversal answers "does a path exist?"
- Constraints answer "are there overriding conditions?"
- Evaluation order: Graph ALLOW → Constraint check → Final decision
- Constraints are registered as Go functions, not stored in DB
- This keeps the graph clean and constraints extensible

### D5: Scoped Share OAs for Cross-Company Sharing

**Choice: Create per-share Object Attributes under SharedDocs (e.g., `Share_Acme_to_Beta_Eng`)**

- Revoking a share = removing one OA assignment, no side effects on other shares
- Clear audit trail — each share is a distinct graph node
- Slightly more graph nodes, but vastly cleaner than a flat SharedDocs OA

**Alternative:** Single flat SharedDocs OA where all shared docs and all target UAs connect → revoking one company's access while keeping another's requires careful edge management

### D6: Frontend Architecture

**Choice: React + Vite SPA with React Router and Zustand for state**

- Vite for fast dev/build
- React Router for client-side routing
- Zustand for lightweight global state (auth token, current user)
- No heavy framework needed — this is a demo/reference app

### D7: File Storage

**Choice: Local filesystem in a Docker volume**

- Files stored at `/data/documents/{doc-id}/{filename}`
- Metadata (title, type, owner, NGAC node ID) in PostgreSQL
- Sufficient for MVP; can swap to S3/MinIO later via storage interface

## Risks / Trade-offs

- **[In-memory graph stale on crash]** → Mitigated by full rebuild from PostgreSQL on startup; acceptable for demo
- **[No horizontal scaling]** → Single backend instance; acceptable for reference implementation
- **[Seed data complexity]** → Complex multi-tenant seed data needed; mitigated by dedicated seed.go with clear comments
- **[PostgreSQL recursive CTE performance]** → At demo scale (hundreds of nodes) this is not an issue; would need benchmarking above 10K nodes
- **[Time-based constraints are server-clock dependent]** → Document this limitation; production would use client timezone or configurable timezone

## Architecture Overview

```
┌──────────────┐     ┌──────────────────────────────────────┐
│   React SPA  │────▶│           Go Backend (Chi)            │
│   (Vite)     │     │                                      │
│   Port 3000  │     │  ┌─────────┐  ┌──────────────────┐  │
│              │     │  │ API     │  │  NGAC Engine      │  │
│  - Login     │     │  │ Layer   │──│  - Graph (mem)    │  │
│  - Dashboard │     │  │         │  │  - Access Check   │  │
│  - Upload    │     │  │ Auth    │  │  - Explain        │  │
│  - Share     │     │  │ Middle  │  │  - Constraints    │  │
│  - Approve   │     │  │ ware    │  │  - Store (pg)     │  │
│  - Explain   │     │  └─────────┘  └──────────────────┘  │
│              │     │                                      │
│              │     │  Port 8080                           │
└──────────────┘     └──────────────┬───────────────────────┘
                                    │
                     ┌──────────────▼───────────────────────┐
                     │         PostgreSQL 16                 │
                     │                                      │
                     │  ngac_nodes, ngac_assignments,       │
                     │  ngac_associations, ngac_prohibitions│
                     │  users, documents                    │
                     │                                      │
                     │  Port 5432                           │
                     └──────────────────────────────────────┘
```
