# Quick Test Environment Setup

## Unified Management Script

All test environment management is now handled by a single script: `test_env.sh`

### Quick Start

**Start everything:**
```bash
make test-env-start
# or directly:
./test/docker/test_env.sh start
```

**Run tests:**
```bash
make test-integration
```

**Check status:**
```bash
make test-env-status
# or:
./test/docker/test_env.sh status
```

**View logs:**
```bash
make test-env-logs
# or:
./test/docker/test_env.sh logs
```

**Stop everything:**
```bash
make test-env-stop
# or:
./test/docker/test_env.sh stop
```

**Restart:**
```bash
make test-env-restart
# or:
./test/docker/test_env.sh restart
```

## Available Commands

The `test_env.sh` script supports these commands:

- `start` - Start GitLab and bot containers
- `stop` - Stop all containers and clean up
- `restart` - Stop and start the environment
- `status` - Show status of all components
- `logs` - Show recent logs from containers

### Start Options

```bash
./test/docker/test_env.sh start --gitlab-version 16.0.0
./test/docker/test_env.sh start --cpus 2 --memory 4g
```

## How It Works

The setup runs GitLab and the bot as separate Docker containers on the default bridge network:
- GitLab container runs as `gitlab-mr-conform-test`
- Bot container runs as `gitlab-mr-conformity-bot`

## View Logs

```bash
# GitLab logs
docker logs -f gitlab-mr-conform-test

# Bot logs  
docker logs -f gitlab-mr-conformity-bot
```

## Troubleshooting

**Check what's running:**
```bash
make test-env-status
```

**View logs:**
```bash
# All logs
make test-env-logs

# Follow specific container logs
docker logs -f gitlab-mr-conform-test    # GitLab
docker logs -f gitlab-mr-conformity-bot  # Bot
```

**Webhook not working?**
```bash
# Check bot health
curl http://localhost:8080/health

# Restart everything
make test-env-restart
```

**Need to rebuild bot after code changes?**
```bash
make test-env-restart
```

**Complete cleanup:**
```bash
make gitlab-clean
```

## Legacy Commands

For compatibility, these still work:
- `make gitlab-start` - Same as `test-env-start`
- `make gitlab-stop` - Same as `test-env-stop`

## Configuration

The bot configuration is in [bot-config.yaml](bot-config.yaml) with all rules enabled for comprehensive testing.
