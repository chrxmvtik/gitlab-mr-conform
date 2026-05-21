#!/usr/bin/env bash

# Quick script to run gitlab-mr-conform bot alongside GitLab test instance

set -e

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
repo_root_dir=$(cd "$script_dir/../.." &>/dev/null && pwd)

echo "Building gitlab-mr-conform Docker image..."
cd "$repo_root_dir"
docker build -t gitlab-mr-conform:test .

echo "Starting gitlab-mr-conform bot..."
docker run -d \
  --name mr-conform-bot \
  --network host \
  -e GITLAB_MR_BOT_GITLAB_TOKEN=token-string-here123 \
  -e GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=test-webhook-secret \
  -e GITLAB_MR_BOT_GITLAB_BASE_URL=http://localhost \
  -e GITLAB_MR_BOT_SERVER_PORT=8081 \
  -v "$script_dir/bot-config.yaml:/configs/config.yaml:ro" \
  gitlab-mr-conform:test

echo ""
echo "✓ Bot is running as 'mr-conform-bot'"
echo "✓ Webhook endpoint: http://localhost:8081/webhook"
echo ""
echo "To view logs:"
echo "  docker logs -f mr-conform-bot"
echo ""
echo "To stop:"
echo "  docker stop mr-conform-bot && docker rm mr-conform-bot"
