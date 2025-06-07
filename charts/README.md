## Installation Instructions

1. **Add helm repository:**

   ```bash
   helm repo add gitlab-mr-conform https://chrxmvtik.github.io/gitlab-mr-conform/
   ```

2. **Customize values.yaml with your secrets:**

   ```bash
   echo -n "your-gitlab-token" | base64
   echo -n "your-webhook-secret" | base64
   ```

3. **Install the chart:**

   ```bash
   # Install with custom values
   helm upgrade --install gitlab-mr-conform gitlab-mr-conform/gitlab-mr-conform --version 0.0.1 \
     --set secret.data.gitlabToken="eW91ci1naXRsYWItdG9rZW4=" \
     --set secret.data.webhookSecret="eW91ci13ZWJob29rLXNlY3JldA==" \
     --set replicaCount=3

   # Or customize configuration rules
   helm upgrade --install gitlab-mr-conform gitlab-mr-conform/gitlab-mr-conform --version 0.0.1 \
     --set config.data.rules.title.min_length=15 \
     --set config.data.rules.description.min_length=30 \
     --set config.data.gitlab.base_url="https://your-gitlab-instance.com"
   ```

## Key Features

- **Configurable replicas** - Adjust the number of pod replicas
- **Flexible image settings** - Customize image repository, tag, and pull policy
- **Service configuration** - Choose service type (LoadBalancer, ClusterIP, NodePort)
- **Resource management** - Set CPU and memory limits/requests
- **Health checks** - Configurable liveness and readiness probes
- **Secret management** - Option to create secrets or use existing ones
- **Configuration management** - Full application config via ConfigMap with customizable rules
- **Security contexts** - Pod and container security settings
- **Node scheduling** - Node selectors, tolerations, and affinity rules
- **Standard Helm practices** - Proper labeling, naming, and templating

## Environment Variables

The chart automatically sets up the required environment variables:

- `GITLAB_MR_BOT_GITLAB_TOKEN` - GitLab API token
- `GITLAB_MR_BOT_GITLAB_SECRET_TOKEN` - Webhook secret token

These are sourced from the Kubernetes secret created by the chart.

## Configuration Customization

The application configuration can be customized in several ways:

**1. Via values.yaml:**

```yaml
config:
  data:
    gitlab:
      base_url: "https://your-gitlab-instance.com"
    rules:
      title:
        min_length: 15
        max_length: 120
        conventional:
          types:
            - "feat"
            - "fix"
            - "docs"
            - "style"
            - "refactor"
      description:
        min_length: 30
        require_template: true
```

**2. Via --set flags:**

```bash
helm upgrade --install gitlab-mr-conform gitlab-mr-conform/gitlab-mr-conform --version 0.0.1 \
  --set config.data.rules.title.min_length=15 \
  --set config.data.rules.description.min_length=30 \
  --set config.data.gitlab.base_url="https://your-gitlab.com"
```

**3. Via separate values file:**

```bash
# Create custom-values.yaml with your overrides
helm upgrade --install gitlab-mr-conform gitlab-mr-conform/gitlab-mr-conform --version 0.0.1 -f custom-values.yaml
```

The ConfigMap will be mounted at `/app/config.yaml` inside the container, making it available to the application.
