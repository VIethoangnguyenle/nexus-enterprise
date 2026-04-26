## ADDED Requirements

### Requirement: Company management
The system SHALL provide API endpoints to list all companies (Policy Classes) and their departments (User Attributes).

#### Scenario: List companies
- **WHEN** an authenticated user calls GET /api/companies
- **THEN** the system returns all Policy Classes (excluding PC_Global)

#### Scenario: List departments
- **WHEN** a user calls GET /api/companies/:id/departments
- **THEN** the system returns all UAs assigned to that PC

### Requirement: Department creation
The system SHALL allow creating new departments under an existing company by creating a UA node and assigning it to the company's PC.

#### Scenario: Create department
- **WHEN** an admin creates department "Legal" under PC_Acme
- **THEN** a UA "Acme_Legal" is created and assigned to PC_Acme, plus default OA and associations are set up

### Requirement: User listing and management
The system SHALL provide endpoints to list users with their company and department assignments.

#### Scenario: List users
- **WHEN** an admin calls GET /api/users
- **THEN** the system returns all users with their UA memberships
