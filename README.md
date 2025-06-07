# GitLab MR Conform Checker

## 🧭 Overview

**GitLab MR Conform Checker** is an automated tool designed to enforce compliance and quality standards on GitLab merge requests (MRs). By programmatically validating MRs against organizational rules, it reduces human error, ensures consistency, and accelerates code reviews. It integrates directly with GitLab and leaves a structured discussion on each MR highlighting any conformity violations.

## 🚀 Features

- 🔎 **MR Title & Description Validation**: Enforces format (e.g., JIRA key), length, and structure.
- 💬 **Commit Message Checks**: Ensures message compliance with standards (e.g., Conventional Commits).
- 🏷️ **JIRA Issue Linking**: Verifies associated issue keys in MRs or commits.
- 🌱 **Branch Rules**: Validates naming conventions (e.g., `feature/`, `bugfix/`, `hotfix/`).
- 📦 **Squash Commit Enforcement**: Checks MR squash settings when required.
- 👥 **Approval Rules**: Ensures required reviewers have approved the MR.
- 🛠️ **Extensible Rules Engine**: Easily add custom checks or adjust rule strictness per project.

### 📝 Automated Reporting

- Creates structured discussions on merge requests with violation details
- Provides clear, actionable feedback for developers
- Tracks compliance status across projects

## 🚀 Quick Start

### 1. Installation

**Prerequisites:** Go 1.21+ and GitLab API access token

```bash
# Clone and build
make build
```

### 2. Configuration

Set up your environment:

```bash
export GITLAB_MR_BOT_GITLAB_TOKEN="your_gitlab_token"
export GITLAB_MR_BOT_GITLAB_SECRET_TOKEN="your_webhook_secret"
```

Create a `config.yaml` file to define your compliance rules:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

gitlab:
  base_url: "https://gitlab.com"

rules:
  title:
    enabled: true
    min_length: 10
    max_length: 100
    conventional:
      types: ["feat", "fix", "docs", "refactor", "release"]
    jira:
      keys: ["PROJ", "JIRA"]

  description:
    enabled: true
    required: true
    min_length: 20

  branch:
    enabled: true
    allowed_prefixes: ["feature/", "bugfix/", "hotfix/", "release/"]
    forbidden_names: ["master", "main", "develop"]

  commits:
    enabled: true
    max_length: 72
    conventional:
      types: ["feat", "fix", "docs", "refactor", "release"]

  approvals:
    enabled: false
    required: false
    min_count: 1

  squash:
    enabled: true
    enforce_branches: ["feature/*", "fix/*"]
```

### 3. Setup GitLab Webhook

1. Navigate to your GitLab project → **Settings** → **Webhooks**
2. Add webhook:
   - **URL:** `https://your-domain.com/webhook`
   - **Trigger:** Merge request events
   - **Secret Token:** Your webhook secret
3. Start the service: `make run`

## Example Output

## 🧾 **MR Conformity Check Summary**

### ❌ 1 conformity check(s) failed:

---

#### ⚠️ **Commit Messages**

📄 **Issue 1**: 3 commit(s) have invalid Conventional Commit format:

- Merge branch 'security-300265-13-18' into '13-18-s... ([d6b32537](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/d6b32537346c98c21f25a84e9bd060c6a9188fec))
- Update CHANGELOG and VERSION ([be84773e](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/be84773e180914570ef2af88c839df3d26149153))
- Modify regex to prevent partial matches ([1f04c93c](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/1f04c93c90cb44c805040def751d2753a7f16f29))
  > 💡 **Tip**: Use format:
  >
  > ```
  > type(scope?): description
  > ```
  >
  > Example:
  > `feat(auth): add login retry mechanism`

## 🐳 Deployment Options

### Docker

```bash
docker run -p 8080:8080 \
  -e GITLAB_MR_BOT_GITLAB_TOKEN=$GITLAB_TOKEN \
  -e GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=$WEBHOOK_SECRET \
  ghcr.io/chrxmvtik/gitlab-mr-conform:latest
```

### Docker Compose

```yaml
version: "3.8"
services:
  mr-checker:
    image: ghcr.io/chrxmvtik/gitlab-mr-conform:latest
    ports:
      - "8080:8080"
    environment:
      - GITLAB_MR_BOT_GITLAB_TOKEN=${GITLAB_TOKEN}
      - GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=${WEBHOOK_SECRET}
    volumes:
      - ./config.yaml:/app/config.yaml
```

### Kubernetes/Helm

Deploy using our:

- Helm chart - see [charts/README.md](charts/README.md) for details.
- Plain manifest - [manifest](deploy/k8s/manifest.yaml)

## 🔧 API Reference

| Endpoint   | Method | Description                  |
| ---------- | ------ | ---------------------------- |
| `/webhook` | POST   | GitLab webhook receiver      |
| `/health`  | GET    | Health check                 |
| `/status`  | GET    | Merge request status checker |

## 🧪 Development

```bash
# Setup development environment
make dev-setup

# Run tests
make test

# Run locally
make run

# Build for production
make build
```

## 🔍 Troubleshooting

**Webhook not receiving events?**

- Verify GitLab can reach your endpoint
- Check webhook secret configuration
- Review GitLab webhook logs

**False positive violations?**

- Adjust rule strictness in `config.yaml`
- Review regex patterns for validation
- Test rules against existing MRs
