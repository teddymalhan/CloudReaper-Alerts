#!/usr/bin/env bash
# End-to-end local demo: provision on Floci (AWS emulator), scan for orphans, and (if any) push
# a Slack alert through the pipeline. Pass --dry-run to skip the final POST and print the report.
#
#   SLACK_BOT_TOKEN / SLACK_CHANNEL_ID  — set these for a real Slack message (else placeholders).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN:-xoxb-localstack-placeholder}"
SLACK_CHANNEL_ID="${SLACK_CHANNEL_ID:-C0000000000}"
DRY_RUN=0
[ "${1:-}" = "--dry-run" ] && DRY_RUN=1

# 1. Floci (AWS emulator) -------------------------------------------------------
if ! curl -s -o /dev/null "$ENDPOINT" 2>/dev/null; then
  echo "==> Starting Floci (docker compose)"
  docker compose up -d floci
fi
echo -n "    waiting for Floci on $ENDPOINT"
for _ in $(seq 1 45); do
  curl -s -o /dev/null "$ENDPOINT" 2>/dev/null && break
  echo -n "."; sleep 2
done
echo

# 2. Build the notifier Lambda (Linux bootstrap zip) ----------------------------
echo "==> Building notifier Lambda"
mkdir -p terraform/build build
GOOS=linux GOARCH="$(go env GOARCH)" CGO_ENABLED=0 \
  go build -tags lambda.norpc -o terraform/build/bootstrap ./cmd/notifier
( cd terraform/build && rm -f notifier.zip && zip -q notifier.zip bootstrap )

# 3. Provision -------------------------------------------------------------------
echo "==> terraform apply"
terraform -chdir=terraform init -input=false >/dev/null
terraform -chdir=terraform apply -auto-approve -input=false \
  -var "slack_bot_token=$SLACK_BOT_TOKEN" \
  -var "slack_channel_id=$SLACK_CHANNEL_ID"

# 4. Detect ----------------------------------------------------------------------
echo "==> Running detector"
go build -o build/detector ./cmd/detector
set +e
AWS_ENDPOINT_URL="$ENDPOINT" ./build/detector -report build/report.json -markdown build/report.md
ORPHANS=$?
set -e
echo
cat build/report.md
echo

if [ "$ORPHANS" -eq 0 ]; then
  echo "==> No orphans found — nothing to notify."
  exit 0
fi

# 5. Notify ----------------------------------------------------------------------
SEND_URL="$(terraform -chdir=terraform output -raw send_message_endpoint)"
if [ "$DRY_RUN" -eq 1 ]; then
  echo "==> [dry-run] orphans found; would POST report to:"
  echo "    $SEND_URL"
  exit 0
fi

echo "==> Sending orphan alert to $SEND_URL"
go build -o build/sender ./cmd/sender
./build/sender -report build/report.json -endpoint "$SEND_URL" -build-url "${BUILD_URL:-local-run}"
echo "==> Alert queued. Check the notifier Lambda logs and your Slack channel."
