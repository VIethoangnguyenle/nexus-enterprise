#!/bin/bash
# Start all NGAC backend services from their individual directories
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
set -a; source /home/zane/Desktop/ngac/.env.dev; set +a
LOGDIR=/home/zane/Desktop/ngac/.dev-logs
mkdir -p $LOGDIR

cd /home/zane/Desktop/ngac/backend/services/policy
DATABASE_URL=$DATABASE_URL REDIS_URL=$REDIS_URL_POLICY KAFKA_BROKERS=$KAFKA_BROKERS GRPC_PORT=50051 \
  go run ./cmd/ > $LOGDIR/policy.log 2>&1 &
echo "policy=$!"
sleep 3

cd /home/zane/Desktop/ngac/backend/services/auth
DATABASE_URL=$DATABASE_URL REDIS_URL=$REDIS_URL_AUTH POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  WORKSPACE_SERVICE_ADDR=$WORKSPACE_SERVICE_ADDR MESSAGING_SERVICE_ADDR=$MESSAGING_SERVICE_ADDR \
  JWT_SECRET=$JWT_SECRET GRPC_PORT=50052 REST_PORT=$AUTH_REST_PORT \
  go run ./cmd/ > $LOGDIR/auth.log 2>&1 &
echo "auth=$!"

cd /home/zane/Desktop/ngac/backend/services/workspace
DATABASE_URL=$DATABASE_URL POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  AUTH_SERVICE_ADDR=$AUTH_SERVICE_ADDR DRIVE_SERVICE_ADDR=$DRIVE_SERVICE_ADDR \
  MESSAGING_SERVICE_ADDR=$MESSAGING_SERVICE_ADDR GRPC_PORT=50053 REST_PORT=$WORKSPACE_REST_PORT \
  go run ./cmd/ > $LOGDIR/workspace.log 2>&1 &
echo "workspace=$!"

cd /home/zane/Desktop/ngac/backend/services/document
DATABASE_URL=$DATABASE_URL POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  GRPC_PORT=50054 REST_PORT=$DOCUMENT_REST_PORT \
  go run ./cmd/ > $LOGDIR/document.log 2>&1 &
echo "document=$!"

cd /home/zane/Desktop/ngac/backend/services/messaging
DATABASE_URL=$DATABASE_URL REDIS_URL=$REDIS_URL_MESSAGING POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  WORKSPACE_SERVICE_ADDR=$WORKSPACE_SERVICE_ADDR AUTH_SERVICE_ADDR=$AUTH_SERVICE_ADDR \
  GRPC_PORT=50055 REST_PORT=$MESSAGING_REST_PORT WS_PORT=$WS_PORT \
  go run ./cmd/ > $LOGDIR/messaging.log 2>&1 &
echo "messaging=$!"

cd /home/zane/Desktop/ngac/backend/services/asset
DATABASE_URL=$DATABASE_URL POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  GRPC_PORT=50056 REST_PORT=$ASSET_REST_PORT \
  go run ./cmd/ > $LOGDIR/asset.log 2>&1 &
echo "asset=$!"

cd /home/zane/Desktop/ngac/backend/services/drive
DATABASE_URL=$DATABASE_URL REDIS_URL=$REDIS_URL_DRIVE POLICY_SERVICE_ADDR=$POLICY_SERVICE_ADDR \
  WORKSPACE_SERVICE_ADDR=$WORKSPACE_SERVICE_ADDR MESSAGING_SERVICE_ADDR=$MESSAGING_SERVICE_ADDR \
  MINIO_ENDPOINT=$MINIO_ENDPOINT MINIO_ACCESS_KEY=$MINIO_ACCESS_KEY MINIO_SECRET_KEY=$MINIO_SECRET_KEY \
  GRPC_PORT=50057 REST_PORT=$DRIVE_REST_PORT \
  go run ./cmd/ > $LOGDIR/drive.log 2>&1 &
echo "drive=$!"

echo "All services starting..."
