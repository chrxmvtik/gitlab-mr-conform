#!/usr/bin/env bash

echo "Stopping gitlab-mr-conform bot..."
docker stop gitlab-mr-conformity-bot 2>/dev/null || true
docker rm gitlab-mr-conformity-bot 2>/dev/null || true
echo "✓ Bot stopped"
