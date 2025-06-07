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
- 🗣️ **Automated MR Discussions**: Posts a detailed comment listing all conformity violations.
- 🛠️ **Extensible Rules Engine**: Easily add custom checks or adjust rule strictness per project.

### Automated Reporting

- Creates structured discussions on merge requests with violation details
- Provides clear, actionable feedback for developers
- Tracks compliance status across projects

## Installation

### Prerequisites

- Go 1.21+ (for development only)
- GitLab API access token with appropriate permissions

## Configuration

### Environment Variables

```bash
# GitLab Configuration
GITLAB_MR_BOT_GITLAB_TOKEN=gitlab_token
GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=webhook_token
```

### Rules Configuration

Create a `config.yaml` file to define your compliance rules:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

gitlab:
  # Set via environment variables:
  # GITLAB_MR_BOT_GITLAB_TOKEN
  # GITLAB_MR_BOT_GITLAB_SECRET_TOKEN
  base_url: "https://gitlab.com"

rules:
  title:
    enabled: true
    min_length: 10
    max_length: 100
    conventional:
      types:
        - "feat"
        - "fix"
        - "docs"
        - "refactor"
        - "release"
      scopes:
        - ".*"
    forbidden_words:
      - "WIP"
      - "TODO"
      - "FIXME"
    jira:
      keys:
        - PROJ
        - JIRA

  description:
    enabled: true
    required: true
    min_length: 20
    require_template: false

  branch:
    enabled: true
    allowed_prefixes: ["feature/", "bugfix/", "hotfix/", "release/"]
    forbidden_names: ["master", "main", "develop", "staging"]

  commits:
    enabled: true
    max_length: 72
    conventional:
      types:
        - "feat"
        - "fix"
        - "docs"
        - "refactor"
        - "release"
      scopes:
        - ".*"
    jira:
      keys: []

  approvals:
    enabled: false
    required: false
    min_count: 1

  squash:
    enabled: true
    enforce_branches:
      - "feature/*"
      - "fix/*"
    disallow_branches: ["release/*", "hotfix/*"]
```

## Usage

### Webhook Integration

1. Set up a GitLab webhook pointing to your service endpoint:

   - URL: `https://your-domain.com/webhook`
   - Trigger: Merge request events
   - Secret Token: Your webhook secret

2. Start the service:

```bash
make run
# or if built:
./bin/gitlab-mr-conform
```

## API Endpoints

### Webhook Endpoint

- `POST /webhook` - Receives GitLab webhook events

### Status Endpoints

- `GET /health` - Health check endpoint
- `GET /status` - Service status and statistics

## Example Output

## :receipt: **MR Conformity Check Summary**

### :x: 4 conformity check(s) failed:

---

#### :x: **Title Validation**

:page_facing_up: **Issue 1**: No Jira issue tag found in title: "feat: shit shittest"

> :bulb: **Tip**: Include a Jira tag like \[ABC-123\] or ABC-123\
> **Example**:\
> `fix(token): handle expired JWT refresh logic [SEC-456]`

---

---

#### :warning: **Description Validation**

:page_facing_up: **Issue 1**: Description too short (minimum 20 characters)

> :bulb: **Tip**: Provide more details about the changes

---

---

#### :warning: **Branch Naming**

:page_facing_up: **Issue 1**: Branch should start with: feature/, bugfix/, hotfix/, release/

> :bulb: **Tip**: Rename branch to start with 'feature/'

---

---

#### :warning: **Commit Messages**

:page_facing_up: **Issue 1**: 3 commit(s) have invalid Conventional Commit format:

- Merge branch 'security-300265-13-18' into '13-18-s... ([d6b32537](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/d6b32537346c98c21f25a84e9bd060c6a9188fec))
- Update CHANGELOG and VERSION ([be84773e](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/be84773e180914570ef2af88c839df3d26149153))
- Modify regex to prevent partial matches ([1f04c93c](http://0.0.0.0:3000/gitlab-org/gitlab-shell/-/commit/1f04c93c90cb44c805040def751d2753a7f16f29))

> :bulb: **Tip**: Use format:
>
> ```
> type(scope?): description
> ```
>
> Example: `feat(auth): add login retry mechanism`

---

---

## Development

### Running Tests

```bash
# Install packages
make dev-setup

# Run all tests
make test

# Run application
make run
```

## Deployment

### Docker

#### Run Container

```bash
	docker run -p 8080:8080 \
		-e GITLAB_MR_BOT_GITLAB_TOKEN=$(GITLAB_MR_BOT_GITLAB_TOKEN) \
		-e GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=$(GITLAB_MR_BOT_GITLAB_SECRET_TOKEN) \
		$(APP_NAME):$(VERSION)
```

### Docker Compose

Example compose file is available [here](deploy/docker/compose.yaml)

### Helm Chart

Deploy to Kubernetes using Helm, see instructions [here](charts/README.md)

## Troubleshooting

### Common Issues

**Issue**: Webhook not receiving events

- Check GitLab webhook configuration
- Verify URL is accessible from GitLab
- Check webhook secret configuration

**Issue**: False positive conformity violations

- Review and adjust rules in `conformity-rules.yml`
- Check regex patterns for title/commit validation
- Verify branch naming conventions
