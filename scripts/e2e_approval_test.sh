#!/bin/bash
# E2E Verification Script for Multi-Tenant Approval Engine
# Prerequisites: PostgreSQL, Redis, Redpanda, all services running
#
# Usage:
#   chmod +x scripts/e2e_approval_test.sh
#   ./scripts/e2e_approval_test.sh
#
# Environment variables:
#   API_BASE     - Base URL for approval REST API (default: http://localhost:8080)
#   POLICY_ADDR  - Policy service gRPC address (default: localhost:50051)

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080}"
PASS=0
FAIL=0
TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); echo "  ✅ $1"; }
fail() { ((FAIL++)); ((TOTAL++)); echo "  ❌ $1: $2"; }

header() { echo ""; echo "=== $1 ==="; }

# --- Test 1: Schema Isolation ---
header "Test 1: Schema Isolation"

# Create tenant A
RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/tenants" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "tenant_a", "name": "Tenant A"}')
if [ "$RESP" = "200" ] || [ "$RESP" = "201" ]; then pass "Create tenant A"; else fail "Create tenant A" "HTTP $RESP"; fi

# Create tenant B
RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/tenants" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "tenant_b", "name": "Tenant B"}')
if [ "$RESP" = "200" ] || [ "$RESP" = "201" ]; then pass "Create tenant B"; else fail "Create tenant B" "HTTP $RESP"; fi

# --- Test 2: Template CRUD ---
header "Test 2: Template CRUD (Tenant A)"

TEMPLATE=$(curl -s -X POST "${API_BASE}/api/approval/templates" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d '{
    "name": "High Value Transfer",
    "entity_type": "transfer",
    "is_active": true,
    "priority": 10,
    "conditions": [{"field": "amount", "operator": "gt", "value": "1000000"}],
    "steps": [
      {"step_order": 1, "name": "Manager", "approver_type": "specific_user", "approver_value": "approver1", "required_count": 1},
      {"step_order": 2, "name": "Director", "approver_type": "specific_user", "approver_value": "approver2", "required_count": 1},
      {"step_order": 3, "name": "CEO", "approver_type": "specific_user", "approver_value": "approver3", "required_count": 1}
    ]
  }')
TEMPLATE_ID=$(echo "$TEMPLATE" | jq -r '.id // empty')
if [ -n "$TEMPLATE_ID" ]; then pass "Create 3-step template: $TEMPLATE_ID"; else fail "Create template" "$TEMPLATE"; fi

# --- Test 3: Approval Flow ---
header "Test 3: 3-Step Approval Flow"

REQUEST=$(curl -s -X POST "${API_BASE}/api/approval/requests" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d "{
    \"entity_type\": \"transfer\",
    \"entity_id\": \"txn-001\",
    \"entity_fields\": {\"amount\": \"5000000\"},
    \"scope_oa_id\": \"dept_oa_1\",
    \"department_id\": \"dept1\",
    \"created_by\": \"user1\"
  }")
REQUEST_ID=$(echo "$REQUEST" | jq -r '.id // empty')
if [ -n "$REQUEST_ID" ]; then pass "Create request: $REQUEST_ID"; else fail "Create request" "$REQUEST"; fi

# Step 1: Manager approves
RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/approval/approve" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d "{\"request_id\": \"$REQUEST_ID\", \"user_node_id\": \"approver1\", \"comment\": \"ok step 1\"}")
if [ "$RESP" = "200" ]; then pass "Step 1 approve"; else fail "Step 1 approve" "HTTP $RESP"; fi

# Step 2: Director approves
RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/approval/approve" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d "{\"request_id\": \"$REQUEST_ID\", \"user_node_id\": \"approver2\", \"comment\": \"ok step 2\"}")
if [ "$RESP" = "200" ]; then pass "Step 2 approve"; else fail "Step 2 approve" "HTTP $RESP"; fi

# Step 3: CEO approves
RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/approval/approve" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d "{\"request_id\": \"$REQUEST_ID\", \"user_node_id\": \"approver3\", \"comment\": \"ok step 3\"}")
if [ "$RESP" = "200" ]; then pass "Step 3 approve (final)"; else fail "Step 3 approve" "HTTP $RESP"; fi

# Verify completed
STATUS=$(curl -s "${API_BASE}/api/approval/requests/${REQUEST_ID}" \
  -H "X-Tenant-ID: tenant_a" | jq -r '.status // empty')
if [ "$STATUS" = "approved" ]; then pass "Request completed: approved"; else fail "Request status" "got $STATUS, want approved"; fi

# --- Test 4: Reject Flow ---
header "Test 4: Reject = Terminal"

REQUEST2=$(curl -s -X POST "${API_BASE}/api/approval/requests" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant_a" \
  -d "{
    \"entity_type\": \"transfer\",
    \"entity_id\": \"txn-002\",
    \"entity_fields\": {\"amount\": \"2000000\"},
    \"scope_oa_id\": \"dept_oa_1\",
    \"department_id\": \"dept1\",
    \"created_by\": \"user1\"
  }")
REQUEST2_ID=$(echo "$REQUEST2" | jq -r '.id // empty')

if [ -n "$REQUEST2_ID" ]; then
  # Reject at step 1
  RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/approval/reject" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: tenant_a" \
    -d "{\"request_id\": \"$REQUEST2_ID\", \"user_node_id\": \"approver1\", \"comment\": \"rejected\"}")
  if [ "$RESP" = "200" ]; then pass "Reject at step 1"; else fail "Reject" "HTTP $RESP"; fi

  STATUS=$(curl -s "${API_BASE}/api/approval/requests/${REQUEST2_ID}" \
    -H "X-Tenant-ID: tenant_a" | jq -r '.status // empty')
  if [ "$STATUS" = "rejected" ]; then pass "Request terminal: rejected"; else fail "Terminal status" "got $STATUS"; fi
fi

# --- Test 5: Batch Approve ---
header "Test 5: Batch Approve"

# Create 3 requests for batch
BATCH_IDS=()
for i in 1 2 3; do
  REQ=$(curl -s -X POST "${API_BASE}/api/approval/requests" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: tenant_a" \
    -d "{
      \"entity_type\": \"transfer\",
      \"entity_id\": \"batch-txn-${i}\",
      \"entity_fields\": {\"amount\": \"3000000\"},
      \"scope_oa_id\": \"dept_oa_1\",
      \"department_id\": \"dept1\",
      \"created_by\": \"user1\"
    }")
  ID=$(echo "$REQ" | jq -r '.id // empty')
  if [ -n "$ID" ]; then BATCH_IDS+=("$ID"); fi
done

if [ ${#BATCH_IDS[@]} -eq 3 ]; then
  IDS_JSON=$(printf '"%s",' "${BATCH_IDS[@]}" | sed 's/,$//')
  RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_BASE}/api/approval/batch-approve" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: tenant_a" \
    -d "{\"request_ids\": [$IDS_JSON], \"user_node_id\": \"approver1\", \"comment\": \"batch ok\"}")
  if [ "$RESP" = "200" ]; then pass "Batch approve 3 items"; else fail "Batch approve" "HTTP $RESP"; fi
fi

# --- Test 6: Audit Trail ---
header "Test 6: Audit Trail"

if [ -n "$REQUEST_ID" ]; then
  AUDIT=$(curl -s "${API_BASE}/api/approval/requests/${REQUEST_ID}/audit" \
    -H "X-Tenant-ID: tenant_a")
  AUDIT_COUNT=$(echo "$AUDIT" | jq 'length // 0')
  if [ "$AUDIT_COUNT" -gt 0 ]; then pass "Audit trail has $AUDIT_COUNT entries"; else fail "Audit trail" "empty"; fi

  # Check for expected actions
  ACTIONS=$(echo "$AUDIT" | jq -r '.[].action' | sort -u | tr '\n' ',')
  if echo "$ACTIONS" | grep -q "created"; then pass "Audit has 'created'"; else fail "Audit" "missing 'created'"; fi
  if echo "$ACTIONS" | grep -q "approved"; then pass "Audit has 'approved'"; else fail "Audit" "missing 'approved'"; fi
fi

# --- Test 7: Cross-Tenant Isolation ---
header "Test 7: Cross-Tenant Isolation"

# Tenant B should not see Tenant A's requests
TENANT_B_PENDING=$(curl -s "${API_BASE}/api/approval/pending?user_node_id=approver1" \
  -H "X-Tenant-ID: tenant_b")
B_COUNT=$(echo "$TENANT_B_PENDING" | jq 'length // 0')
if [ "$B_COUNT" -eq 0 ]; then pass "Tenant B sees 0 requests (isolated)"; else fail "Isolation" "Tenant B sees $B_COUNT requests"; fi

# --- Summary ---
echo ""
echo "================================"
echo " Results: $PASS passed, $FAIL failed, $TOTAL total"
echo "================================"

if [ "$FAIL" -gt 0 ]; then exit 1; fi
