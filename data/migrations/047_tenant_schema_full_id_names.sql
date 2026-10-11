-- provision_tenant_schema() named a tenant's schema from the first 8 characters of
-- its ID, and schema_name is UNIQUE: two workspaces whose IDs begin alike could not
-- both have approvals, and the second one failed on every call. New schemas are now
-- named from the whole ID (lower-case, anything but letters and digits as '_',
-- prefix 'tenant_'; a UUID gives 43 of Postgres's 63 characters).
--
-- A tenant that is already registered keeps the schema_name it has: the name is
-- read from tenant_schemas first, and resolution always goes through that table.
--
-- The function body (007, which also carries the later index changes) is not
-- copied here: its definition is read back and only the line that derives the name
-- is replaced, so the two cannot drift apart. Applying this twice changes nothing.
DO $migration$
DECLARE
    v_def TEXT := pg_get_functiondef('provision_tenant_schema(text)'::regprocedure);
    v_old TEXT := $old$v_schema := 'tenant_' || REPLACE(LEFT(p_tenant_id, 8), '-', '');$old$;
    v_new TEXT := $new$SELECT schema_name INTO v_schema FROM tenant_schemas WHERE tenant_id = p_tenant_id;
    IF v_schema IS NULL THEN
        v_schema := 'tenant_' || lower(regexp_replace(p_tenant_id, '[^a-zA-Z0-9]', '_', 'g'));
        IF length(v_schema) > 63 THEN
            RAISE EXCEPTION 'tenant id too long for a schema name';
        END IF;
    END IF;$new$;
BEGIN
    IF position(v_old IN v_def) = 0 THEN
        IF position('regexp_replace(p_tenant_id' IN v_def) > 0 THEN
            RETURN; -- already applied
        END IF;
        RAISE EXCEPTION 'provision_tenant_schema: the schema-name line was not found';
    END IF;
    EXECUTE replace(v_def, v_old, v_new);
END
$migration$;
