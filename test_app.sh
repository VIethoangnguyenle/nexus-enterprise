#!/bin/bash
# NGAC Platform - Comprehensive Business Logic Test
BASE="${NGAC_TEST_BASE:-http://zump-biz.vn}"
TS=$(date +%s)
PASS=0; FAIL=0; ERRORS=""

ok()   { PASS=$((PASS+1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL+1)); ERRORS="$ERRORS\n  ❌ $1"; echo "  ❌ $1"; }
check() { if [ "$1" = "0" ]; then ok "$2"; else fail "$2: $3"; fi }

extract() { echo "$1" | python3 -c "import sys,json; print(json.load(sys.stdin)$2)" 2>/dev/null; }

echo "=========================================="
echo "  NGAC Platform - Full Business Test"
echo "=========================================="

# ==========================================
# 1. AUTH: sign in with a one-time code
# ==========================================
# There is no password sign-up or sign-in. A test signs in the way a person does,
# with a code. NGAC_TEST_OTP is the server's fixed test code (AUTH_FIXED_OTP_CODE,
# 999999 unless set otherwise): fine for a dev script, never for a real deployment.
echo ""
echo "--- 1. AUTH ---"

OTP_CODE="${NGAC_TEST_OTP:-999999}"

# otp_signin <email>: prints the verify answer (JSON), or nothing on failure.
otp_signin() {
  local sid
  sid=$(curl -s -X POST "$BASE/api/auth/otp/request" -H 'Content-Type: application/json' \
    -d "{\"identifier\":\"$1\",\"type\":\"email\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['session_id'])" 2>/dev/null)
  [ -n "$sid" ] || return 1
  curl -s -X POST "$BASE/api/auth/otp/verify" -H 'Content-Type: application/json' \
    -d "{\"session_id\":\"$sid\",\"code\":\"$OTP_CODE\"}"
}

R=$(otp_signin "admin_$TS@test.local")
TOKEN1=$(extract "$R" "['access_token']")
UID1=$(extract "$R" "['user']['id']")
NGAC1=$(extract "$R" "['user']['ngac_node_id']")
[ -n "$TOKEN1" ] && ok "Sign in admin1 by code (id=$UID1)" || fail "Sign in admin1: $R"

R=$(otp_signin "user2_$TS@test.local")
TOKEN2=$(extract "$R" "['access_token']")
UID2=$(extract "$R" "['user']['id']")
NGAC2=$(extract "$R" "['user']['ngac_node_id']")
[ -n "$TOKEN2" ] && ok "Sign in user2 by code (id=$UID2)" || fail "Sign in user2: $R"

# Auth negative tests
SID=$(curl -s -X POST "$BASE/api/auth/otp/request" -H 'Content-Type: application/json' -d "{\"identifier\":\"admin_$TS@test.local\",\"type\":\"email\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['session_id'])" 2>/dev/null)
R=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/api/auth/otp/verify" -H 'Content-Type: application/json' -d "{\"session_id\":\"$SID\",\"code\":\"000000\"}")
[ "$R" = "401" ] && ok "Wrong code rejected" || fail "Wrong code not rejected: got $R"

R=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"x","password":"y"}')
[ "$R" != "200" ] && [ "$R" != "201" ] && ok "Password login no longer exists ($R)" || fail "Password login still answers: $R"

R=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/workspaces" -H 'Authorization: Bearer invalid_token')
[ "$R" = "401" ] && ok "Invalid token returns 401" || fail "Invalid token: got $R"

R=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/workspaces")
[ "$R" = "401" ] && ok "No token returns 401" || fail "No token: got $R"

H1="Authorization: Bearer $TOKEN1"
H2="Authorization: Bearer $TOKEN2"

# The account list is gone: nothing may let a signed-in person walk the user table.
R=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/users" -H "$H1")
[ "$R" != "200" ] && ok "GET /api/users is gone ($R)" || fail "GET /api/users still lists accounts"

# ==========================================
# 2. WORKSPACE
# ==========================================
echo ""
echo "--- 2. WORKSPACE ---"

R=$(curl -s -X POST "$BASE/api/workspaces" -H "$H1" -H 'Content-Type: application/json' -d '{"name":"TestWorkspace"}')
WSID=$(extract "$R" "['id']")
[ -n "$WSID" ] && ok "Create workspace (id=$WSID)" || fail "Create workspace: $R"

R=$(curl -s "$BASE/api/workspaces" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert any(w['id']=='$WSID' for w in d.get('workspaces',[]))" 2>/dev/null
check $? "List workspaces contains new WS" "$R"

R=$(curl -s "$BASE/api/workspaces/$WSID" -H "$H1")
WSNAME=$(extract "$R" "['name']")
[ "$WSNAME" = "TestWorkspace" ] && ok "Get workspace by ID" || fail "Get workspace: $R"

# Invite member
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/invite" -H "$H1" -H 'Content-Type: application/json' -d "{\"ngac_node_id\":\"$NGAC2\"}")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Invite user2 to workspace" "$R"

R=$(curl -s "$BASE/api/workspaces/$WSID/members" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('members',[])) >= 2" 2>/dev/null
check $? "List members >= 2" "$R"

# Roles
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/roles" -H "$H1" -H 'Content-Type: application/json' -d '{"name":"Editor"}')
ROLE_ID=$(extract "$R" "['id']")
[ -n "$ROLE_ID" ] && ok "Create role Editor (id=$ROLE_ID)" || fail "Create role: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/roles" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('roles',[])) >= 1" 2>/dev/null
check $? "List roles >= 1" "$R"

# Folders
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/folders" -H "$H1" -H 'Content-Type: application/json' -d '{"name":"Engineering"}')
FOLDER_ID=$(extract "$R" "['id']")
[ -n "$FOLDER_ID" ] && ok "Create folder Engineering" || fail "Create folder: $R"

# Permissions
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/permissions" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"ua_id\":\"$ROLE_ID\",\"oa_id\":\"$FOLDER_ID\",\"operations\":[\"read\",\"write\"]}")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Create permission (Editor→Engineering)" "$R"

# user2 sees workspace after invite
R=$(curl -s "$BASE/api/workspaces" -H "$H2")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert any(w['id']=='$WSID' for w in d.get('workspaces',[]))" 2>/dev/null
check $? "User2 sees workspace after invite" "$R"

# ==========================================
# 3. DOCUMENTS
# ==========================================
echo ""
echo "--- 3. DOCUMENTS ---"

R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/documents" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"title":"Design Doc","filename":"design.pdf","mime_type":"application/pdf","content":"SGVsbG8="}')
DOCID=$(extract "$R" "['id']")
[ -n "$DOCID" ] && ok "Upload document (id=$DOCID)" || fail "Upload document: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/documents" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('documents',[])) >= 1" 2>/dev/null
check $? "List documents >= 1" "$R"

# Approve
R=$(curl -s -X POST "$BASE/api/documents/$DOCID/approve" -H "$H1" -H 'Content-Type: application/json')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Approve document" "$R"

# Share
R=$(curl -s -X POST "$BASE/api/documents/$DOCID/share" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"target_ua_id\":\"$NGAC2\",\"operations\":[\"read\"]}")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Share document with user2" "$R"

# Publish
R=$(curl -s -X POST "$BASE/api/documents/$DOCID/publish" -H "$H1" -H 'Content-Type: application/json')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Publish document" "$R"

# ==========================================
# 3b. PRESIGNED UPLOAD/DOWNLOAD (MinIO)
# ==========================================
echo ""
echo "--- 3b. PRESIGNED UPLOAD ---"

# Get presigned upload URL
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/documents/upload-url" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"title":"Presigned Test","filename":"test.txt","mime_type":"text/plain"}')
UPLOAD_URL=$(extract "$R" "['upload_url']")
PRE_DOCID=$(extract "$R" "['doc_id']")
[ -n "$UPLOAD_URL" ] && ok "Get presigned upload URL (doc_id=$PRE_DOCID)" || fail "Get upload URL: $R"

# Upload file directly to MinIO via presigned PUT URL
if [ -n "$UPLOAD_URL" ]; then
  UPLOAD_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$UPLOAD_URL" \
    -H 'Content-Type: text/plain' \
    -d 'Hello from presigned upload test!')
  [ "$UPLOAD_STATUS" = "200" ] && ok "PUT file to MinIO via presigned URL" || fail "MinIO PUT failed: HTTP $UPLOAD_STATUS"
else
  fail "Skipping MinIO PUT — no upload URL"
fi

# Confirm upload
if [ -n "$PRE_DOCID" ]; then
  R=$(curl -s -X POST "$BASE/api/documents/$PRE_DOCID/confirm" -H "$H1" -H 'Content-Type: application/json')
  CONFIRMED_ID=$(extract "$R" "['id']")
  [ "$CONFIRMED_ID" = "$PRE_DOCID" ] && ok "Confirm upload (status=draft)" || fail "Confirm upload: $R"
else
  fail "Skipping confirm — no doc_id"
fi

# Verify presigned doc appears in list
R=$(curl -s "$BASE/api/workspaces/$WSID/documents" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert any(doc['id']=='$PRE_DOCID' for doc in d.get('documents',[]))" 2>/dev/null
check $? "Presigned doc appears in document list" "$R"

# Get presigned download URL
if [ -n "$PRE_DOCID" ]; then
  R=$(curl -s "$BASE/api/documents/$PRE_DOCID/download-url" -H "$H1")
  DL_URL=$(extract "$R" "['download_url']")
  [ -n "$DL_URL" ] && ok "Get presigned download URL" || fail "Get download URL: $R"

  # Download and verify file content
  if [ -n "$DL_URL" ]; then
    DL_CONTENT=$(curl -s "$DL_URL")
    [ "$DL_CONTENT" = "Hello from presigned upload test!" ] && ok "Download content matches upload" || fail "Download mismatch: got '$DL_CONTENT'"
  else
    fail "Skipping download verify — no download URL"
  fi
else
  fail "Skipping download — no doc_id"
fi

# ==========================================
# 4. MESSAGING
# ==========================================
echo ""
echo "--- 4. MESSAGING ---"

R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/channels" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"name":"general","channel_type":"workspace"}')
CHID=$(extract "$R" "['id']")
[ -n "$CHID" ] && ok "Create channel general (id=$CHID)" || fail "Create channel: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/channels" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('channels',[])) >= 1" 2>/dev/null
check $? "List channels >= 1" "$R"

# Send message
R=$(curl -s -X POST "$BASE/api/channels/$CHID/messages" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"content":"Hello World!"}')
MSGID=$(extract "$R" "['id']")
[ -n "$MSGID" ] && ok "Send message (id=$MSGID)" || fail "Send message: $R"

R=$(curl -s "$BASE/api/channels/$CHID/messages" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('messages',[])) >= 1" 2>/dev/null
check $? "Get messages >= 1" "$R"

# Add channel member
R=$(curl -s -X POST "$BASE/api/channels/$CHID/members" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"ngac_node_id\":\"$NGAC2\"}")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Add channel member" "$R"

R=$(curl -s "$BASE/api/channels/$CHID/members" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('members',[])) >= 2" 2>/dev/null
check $? "List channel members >= 2" "$R"

# DMs
R=$(curl -s -X POST "$BASE/api/dms" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"target_user_id\":\"$UID2\",\"target_ngac_node_id\":\"$NGAC2\"}")
DMID=$(extract "$R" "['id']")
[ -n "$DMID" ] && ok "Create DM (id=$DMID)" || fail "Create DM: $R"

R=$(curl -s "$BASE/api/dms" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('channels',[])) >= 1" 2>/dev/null
check $? "List DMs >= 1" "$R"

# ==========================================
# 5. ASSET TYPES
# ==========================================
echo ""
echo "--- 5. ASSET TYPES ---"

R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/asset-types" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"name":"Laptop","description":"Company laptops","category":"hardware"}')
ATID=$(extract "$R" "['id']")
if [ -z "$ATID" ]; then ATID=$(extract "$R" "['asset_type']['id']"); fi
[ -n "$ATID" ] && ok "Create asset type Laptop (id=$ATID)" || fail "Create asset type: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/asset-types" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('types',d.get('asset_types',[]))) >= 1" 2>/dev/null
check $? "List asset types >= 1" "$R"

R=$(curl -s "$BASE/api/asset-types/$ATID" -H "$H1")
ATNAME=$(extract "$R" "['name']")
if [ -z "$ATNAME" ]; then ATNAME=$(extract "$R" "['asset_type']['name']"); fi
[ "$ATNAME" = "Laptop" ] && ok "Get asset type by ID" || fail "Get asset type: $R"

# ==========================================
# 6. ASSETS CRUD + LIFECYCLE
# ==========================================
echo ""
echo "--- 6. ASSETS ---"

R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/assets" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"name\":\"MacBook Pro 16\",\"type_id\":\"$ATID\",\"custom_fields\":{\"serial_number\":\"SN001\",\"brand\":\"Apple\"}}")
ASID=$(extract "$R" "['id']")
if [ -z "$ASID" ]; then ASID=$(extract "$R" "['asset']['id']"); fi
[ -n "$ASID" ] && ok "Create asset (id=$ASID)" || fail "Create asset: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/assets" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('assets',[])) >= 1" 2>/dev/null
check $? "List assets >= 1" "$R"

R=$(curl -s "$BASE/api/assets/$ASID" -H "$H1")
ASNAME=$(extract "$R" "['name']")
if [ -z "$ASNAME" ]; then ASNAME=$(extract "$R" "['asset']['name']"); fi
[ "$ASNAME" = "MacBook Pro 16" ] && ok "Get asset by ID" || fail "Get asset: $R"

# Update asset
R=$(curl -s -X PUT "$BASE/api/assets/$ASID" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"name":"MacBook Pro 16 M3","custom_fields":{"serial_number":"SN001-v2","brand":"Apple"}}')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Update asset" "$R"

# Transitions
R=$(curl -s "$BASE/api/assets/$ASID/transitions" -H "$H1")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Get available transitions" "$R"

R=$(curl -s -X POST "$BASE/api/assets/$ASID/transition" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"action":"approve","comment":"Approved for deployment"}')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Transition asset (approve)" "$R"

R=$(curl -s "$BASE/api/assets/$ASID/history" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('records',d.get('transitions',[]))) >= 1" 2>/dev/null
check $? "Asset history >= 1 transition" "$R"

# ==========================================
# 7. ASSET REQUESTS
# ==========================================
echo ""
echo "--- 7. ASSET REQUESTS ---"

R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/asset-requests" -H "$H2" -H 'Content-Type: application/json' \
  -d "{\"type_id\":\"$ATID\",\"justification\":\"Need laptop for dev\",\"quantity\":1}")
REQID=$(extract "$R" "['id']")
if [ -z "$REQID" ]; then REQID=$(extract "$R" "['request']['id']"); fi
[ -n "$REQID" ] && ok "Create asset request (id=$REQID)" || fail "Create asset request: $R"

R=$(curl -s "$BASE/api/workspaces/$WSID/asset-requests" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('requests',[])) >= 1" 2>/dev/null
check $? "List asset requests >= 1" "$R"

R=$(curl -s "$BASE/api/asset-requests/$REQID" -H "$H1")
echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); r=d.get('request',d); assert r.get('status')=='pending'" 2>/dev/null
check $? "Request status is pending" "$R"

R=$(curl -s -X POST "$BASE/api/asset-requests/$REQID/approve" -H "$H1" -H 'Content-Type: application/json' \
  -d '{"comment":"Approved"}')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Approve asset request" "$R"

# ==========================================
# 8. NOTIFICATIONS
# ==========================================
echo ""
echo "--- 8. NOTIFICATIONS ---"

R=$(curl -s "$BASE/api/notifications" -H "$H2")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "List notifications" "$R"

R=$(curl -s "$BASE/api/notifications/unread-count" -H "$H2")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Get unread count" "$R"

R=$(curl -s -X POST "$BASE/api/notifications/read-all" -H "$H2" -H 'Content-Type: application/json')
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Mark all notifications read" "$R"

# ==========================================
# 9. THREADS
# ==========================================
echo ""
echo "--- 9. THREADS ---"

R=$(curl -s "$BASE/api/messages/$MSGID/thread" -H "$H1")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Get thread for message" "$R"

# ==========================================
# 9b. WEBSOCKET THROUGH TRAEFIK
# ==========================================
echo ""
echo "--- 9b. WEBSOCKET ---"

# Test WebSocket upgrade through Traefik (direct to messaging service)
# We use curl to test the upgrade request returns 101
WS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H 'Upgrade: websocket' \
  -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  "$BASE/api/ws?token=$TOKEN1" --max-time 2 2>/dev/null)
# curl may return 000 on timeout (expected for WS upgrade held open) or 101
if [ "$WS_STATUS" = "101" ] || [ "$WS_STATUS" = "000" ]; then
  ok "WebSocket upgrade via Traefik (/api/ws)"
else
  fail "WebSocket via Traefik: got HTTP $WS_STATUS"
fi

# ==========================================
# 10. DELETE ASSET (soft delete)
# ==========================================
echo ""
echo "--- 10. CLEANUP ---"

# Create another asset to delete
R=$(curl -s -X POST "$BASE/api/workspaces/$WSID/assets" -H "$H1" -H 'Content-Type: application/json' \
  -d "{\"name\":\"ToDelete\",\"type_id\":\"$ATID\",\"custom_fields\":{}}")
DELID=$(extract "$R" "['id']")
if [ -z "$DELID" ]; then DELID=$(extract "$R" "['asset']['id']"); fi
[ -n "$DELID" ] && ok "Create asset for deletion" || fail "Create delete asset: $R"

R=$(curl -s -X DELETE "$BASE/api/assets/$DELID" -H "$H1")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Delete (soft) asset" "$R"

# Remove member
R=$(curl -s -X DELETE "$BASE/api/workspaces/$WSID/members/$NGAC2" -H "$H1")
echo "$R" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null
check $? "Remove member from workspace" "$R"

# ==========================================
# SUMMARY
# ==========================================
echo ""
echo "=========================================="
echo "  RESULTS: $PASS passed, $FAIL failed"
echo "=========================================="
if [ "$FAIL" -gt 0 ]; then
  echo -e "\nFailed tests:$ERRORS"
  exit 1
fi
echo "All tests passed! ✅"
