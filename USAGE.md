# Usage Instructions

## 1. Setup

```bash
# Clone and setup
git clone <repository>
cd gitlab-mr-conformity-bot
make dev-setup
```

## 2. Configuration

Set environment variables:

```bash
export GITLAB_BOT_GITLAB_TOKEN="your-gitlab-access-token"
export GITLAB_BOT_GITLAB_SECRET_TOKEN="webhook-secret-token"
```

## 3. Run locally

```bash
make run
# or
go run ./cmd/bot
```

## 4. Deploy with Docker

```bash
make docker-build
make docker-run
```

## 5. Setup GitLab Webhook

In your GitLab project:

1. Go to Settings → Webhooks
2. URL: `http://your-server:8080/webhook`
3. Secret Token: (same as GITLAB_SECRET_TOKEN)
4. Trigger: Merge request events
5. Enable SSL verification if using HTTPS

## 6. Test

```bash
# Health check
curl http://localhost:8080/health

# Manual status check
curl http://localhost:8080/status/PROJECT_ID/MR_ID
```
