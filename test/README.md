# Test Environment Documentation

This directory contains the integration test suite and Docker-based test environment for GitLab MR Conform.

## Overview

The test environment consists of:
- **GitLab CE** (`mr-conform-gitlab`) - Full GitLab instance for testing
- **MR Conform Bot** (`mr-conform-bot`) - The bot being tested
- **Integration Tests** - Go tests that interact with both containers

Both containers use `--network host` for simplified networking, with the bot running on port 8081 to avoid conflicts with GitLab's internal services.

## Quick Start

```bash
# Start the complete test environment
make test-env-start

# Run integration tests
make test-integration

# Stop the environment
make test-env-stop
```

## Test Environment Architecture

### Containers

1. **GitLab Container** (`mr-conform-gitlab`)
   - Image: `gitlab/gitlab-ce:latest`
   - Network: `host` mode
   - Ports: 80 (HTTP), 443 (HTTPS), 22 (SSH)
   - Credentials: root / mK9JnG7jwYdFcBNoQ3W3
   - API Token: token-string-here123

2. **Bot Container** (`mr-conform-bot`)
   - Built from: Local Dockerfile
   - Network: `host` mode
   - Port: 8081 (to avoid conflict with GitLab's port 8080)
   - Config: `test/docker/bot-config.yaml`

### Network Configuration

Both containers use **host networking** (`--network host`):
- Simplifies container communication
- Bot and GitLab share the host's network stack
- Webhooks use `http://127.0.0.1:8081/webhook` (IPv4 to avoid IPv6 issues)

### GitLab Configuration

The test GitLab instance is configured via `test/docker/gitlab.rb`:
- Allows local webhook requests (required for testing)
- Webhook timeout: 10 seconds
- Outbound requests allowlist includes localhost ranges

## Development Workflow

### Making Code Changes

When you make changes to the bot code, you only need to rebuild and restart the bot container:

```bash
# Quick restart (rebuilds bot image)
./test/docker/stop_bot.sh && ./test/docker/run_bot.sh

# Or using make
make test-env-restart  # Restarts both (slower)
```

### Viewing Logs

```bash
# All logs
make test-env-logs

# Follow specific container logs
docker logs -f mr-conform-gitlab    # GitLab
docker logs -f mr-conform-bot        # Bot
```

### Checking Status

```bash
make test-env-status
```

This shows:
- Container running status
- Health check status
- Network configuration
- Port bindings

## Test Configuration

### GitLab Settings

**Credentials:**
- Username: `root`
- Password: `mK9JnG7jwYdFcBNoQ3W3`
- API Token: `token-string-here123`

**API Endpoint:**
- URL: `http://localhost`
- API: `http://localhost/api/v4`

### Bot Configuration

Located in `test/docker/bot-config.yaml`:
- Server port: 8081 (configurable via `GITLAB_MR_BOT_SERVER_PORT`)
- GitLab base URL: `http://localhost`
- Log level: debug
- Queue: disabled (synchronous processing)

### Environment Variables

The bot container uses these environment variables:
```bash
GITLAB_MR_BOT_GITLAB_TOKEN=token-string-here123
GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=test-webhook-secret
GITLAB_MR_BOT_GITLAB_BASE_URL=http://localhost
GITLAB_MR_BOT_SERVER_PORT=8081
```

## Test Data

### Test Projects

Integration tests create temporary projects:
- Naming pattern: `test-integration-{timestamp}`
- Automatically cleaned up after tests (via `t.Cleanup()`)

The test project has a webhook configured to point to the bot's webhook endpoint.

### Sample Configurations

To test a specific bot configuration the test case should commit a `mr-conform.yaml` file to the test project repository.
