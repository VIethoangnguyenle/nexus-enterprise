# Spec: Self-Contained Schema

## Stories

### S12: Isolate policy DB migrations

As a platform engineer deploying policy-service independently,
I want all DB migrations self-contained in the policy-service repo,
so that I don't need the platform's init.sql.

**Acceptance Criteria:**
- [ ] `migrations/` folder contains only ngac-specific tables
- [ ] `001_core_schema.sql`: ngac_nodes, ngac_assignments, ngac_associations + indexes
- [ ] `002_cache_schema.sql`: ngac_graph_version, ngac_materialized_access + CTE functions
- [ ] `003_operations.sql`: ngac_operations table
- [ ] `004_prohibitions.sql`: ngac_prohibitions table
- [ ] `InitSchema` RPC runs all migrations in order
- [ ] No references to workspace, channel, drive, approval tables
- [ ] Fresh deploy with only policy migrations → service starts successfully

**Proto mapping:** Existing `InitSchema` RPC — no change needed.

---

## Migration Files

### 001_core_schema.sql
```sql
-- Core NGAC graph tables
CREATE TABLE IF NOT EXISTS ngac_nodes (...)
CREATE TABLE IF NOT EXISTS ngac_assignments (...)
CREATE TABLE IF NOT EXISTS ngac_associations (...)
-- Indexes
```

### 002_cache_schema.sql
```sql
-- Cache infrastructure
CREATE TABLE IF NOT EXISTS ngac_graph_version (...)
CREATE TABLE IF NOT EXISTS ngac_materialized_access (...)
-- CTE functions: ngac_ancestors(), ngac_check_access()
```

### 003_operations.sql
```sql
CREATE TABLE IF NOT EXISTS ngac_operations (...)
```

### 004_prohibitions.sql
```sql
CREATE TABLE IF NOT EXISTS ngac_prohibitions (...)
```
