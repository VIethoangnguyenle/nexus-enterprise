## ADDED Requirements

### Requirement: Docker Compose deployment
The system SHALL provide a docker-compose.yml that brings up all services (PostgreSQL, Go backend, React frontend) with a single `docker-compose up` command.

#### Scenario: Full stack starts
- **WHEN** a user runs `docker-compose up`
- **THEN** PostgreSQL starts on port 5432, backend starts on port 8080 (waits for DB), and frontend starts on port 3000 (proxies to backend)

### Requirement: Seed data on first start
The system SHALL automatically seed the database with sample companies, departments, users, documents, and NGAC graph on first startup. Subsequent starts SHALL skip seeding if data exists.

#### Scenario: First startup seeds data
- **WHEN** the backend starts with an empty database
- **THEN** it creates: PC_Acme, PC_Beta, PC_Global, department UAs, sample users (alice, bob, charlie, dave), PublicUsers UA, SharedDocs OA, PublicDocs OA, DraftDocs OA, ApprovedDocs OA, sample documents, and all necessary associations

### Requirement: Backend Dockerfile
The backend SHALL use a multi-stage Docker build: Go build stage for compilation, scratch/alpine stage for the final ~15MB image.

#### Scenario: Backend builds successfully
- **WHEN** `docker build ./backend` is run
- **THEN** a minimal Go binary image is produced

### Requirement: Frontend Dockerfile
The frontend SHALL use a multi-stage Docker build: Node build stage for Vite production build, nginx stage for serving static files.

#### Scenario: Frontend builds successfully
- **WHEN** `docker build ./frontend` is run
- **THEN** an nginx-based image serving the React SPA is produced
