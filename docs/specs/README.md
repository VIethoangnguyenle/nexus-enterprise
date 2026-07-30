# Capability Specs

Long-term memory of what this system does. A plan describes work that will be finished and
forgotten; a spec describes behaviour that outlives it. When a task changes system behaviour,
the spec it touches is updated in the same commit — that is what keeps this directory true.

## Format

One directory per capability, containing `spec.md`:

```markdown
# <capability-name>

## Purpose
<1-2 sentences: why this capability exists>

## Requirements

### Requirement: <statement in SHALL form>
<description>

#### Scenario: <situation name>
- **WHEN** <condition>
- **THEN** <expected result>
```

Rules:

- Specs describe **behaviour, not implementation**. Swapping a library is not a spec change;
  changing what the system guarantees is.
- Capability names are kebab-case and narrow (`batch-access-check`, not `access`). If a name
  needs "and" in it, it is two capabilities.
- A `## Status` section appears only where a spec is known to diverge from the code. It records
  the divergence rather than resolving it; silently editing a spec to match the code destroys
  the only record that a decision was ever made.

## Index

Verified against the codebase on 2026-07-31.

**Authorization and tenancy**

| Capability | State |
|---|---|
| `batch-access-check` | Matches code |
| `tenant-ngac-init` | Matches code |
| `tenant-identity` | Matches code |
| `tenant-auth-flow` | Matches code |

**Drive**

| Capability | State |
|---|---|
| `drive-context-panel` | Matches code |
| `drive-tree-navigation` | Matches code |
| `drive-realtime-sync` | Matches code |
| `drive-permission-engine` | **Open divergence** — cache key omits tenant; see its Status section |

**Layout**

| Capability | State |
|---|---|
| `lark-messaging-layout` | Partly unverifiable by inspection |
| `lark-data-table-layout` | **Partially implemented** — breadcrumb missing in Drive |
| `lark-sidebar-layout` | **Diverges** — 280px and no collapse toggle |

## Not carried over

Two documents under `openspec/specs/` were left where they are: `nexus-auth-flow` and
`nexus-contacts-module`. Despite their location they contain no requirements and no scenarios —
they are screen-by-screen design descriptions with SQL and TSX pasted inline. Reformatting them
would give rot the appearance of rigour. They need rewriting from the behaviour they imply, at
which point they belong here. Note that `nexus-auth-flow` describes an `/_auth/verify` route
that does not exist in `frontend/src/routes/_auth/`.

`openspec/changes/` is a record of past work, not behaviour, and does not belong here at all.
