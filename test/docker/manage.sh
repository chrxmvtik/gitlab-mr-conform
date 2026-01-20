#!/usr/bin/env bash

# ============================================================================
#  GitLab MR Conform Test Environment Manager (Docker Compose)
#
#  This script orchestrates the test environment using Docker Compose
# ============================================================================

set -e

# ============================================================================
#  Functions
# ============================================================================

# Print colored output
cecho() {
  local color=$1 text=$2

  if [[ $TERM == "dumb" ]]; then
    echo "$text"
    return
  fi

  case $(echo "$color" | tr '[:upper:]' '[:lower:]') in
    bk | black) color=0 ;;
    r | red)    color=1 ;;
    g | green)  color=2 ;;
    y | yellow) color=3 ;;
    b | blue)   color=4 ;;
    m | magenta)color=5 ;;
    c | cyan)   color=6 ;;
    w | white | *) color=7 ;; # white or invalid color
  esac

  tput setaf "$color"
  echo -e "$text"
  tput sgr0
}

show_usage() {
  echo "Usage: $0 <command> [options]"
  echo ""
  echo "Commands:"
  echo "  start         Start the test environment (GitLab + bot)"
  echo "  stop          Stop the test environment"
  echo "  restart       Restart the test environment"
  echo "  status        Show status of test environment"
  echo "  logs          Show logs from all containers"
  echo "  logs-gitlab   Show GitLab logs only"
  echo "  logs-bot      Show bot logs only"
  echo ""
  echo "Options for 'start':"
  echo "  --gitlab-version <version>  Specify GitLab version (default: latest)"
  echo "  --cpus <value>              Limit CPU for GitLab (e.g., 0.5, 2)"
  echo "  --memory <value>            Limit memory for GitLab (e.g., 512m, 2g)"
  echo ""
  echo "Examples:"
  echo "  $0 start"
  echo "  $0 start --gitlab-version 16.0.0 --cpus 2 --memory 4g"
  echo "  $0 stop"
  echo "  $0 status"
  echo "  $0 logs -f"
}

# ============================================================================
#  Variables
# ============================================================================

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
repo_root_dir=$(cd "$script_dir/../.." &>/dev/null && pwd)
compose_file="$script_dir/docker-compose.yml"

# ============================================================================
#  Command Functions
# ============================================================================

start_environment() {
  cecho b "=== Starting Test Environment ==="
  echo ""

  # Parse command-line arguments
  local gitlab_version="latest"
  local compose_override=""
  local use_override=false

  while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
      --gitlab-version)
        if [[ -n "$2" ]]; then
          gitlab_version="$2"
          shift 2
        else
          echo "Error: --gitlab-version requires a value"
          exit 1
        fi
        ;;
      --cpus)
        if [[ -n "$2" ]]; then
          use_override=true
          compose_override+="
services:
  gitlab:
    deploy:
      resources:
        limits:
          cpus: '$2'
"
          shift 2
        else
          echo "Error: --cpus requires a value"
          exit 1
        fi
        ;;
      --memory)
        if [[ -n "$2" ]]; then
          use_override=true
          if [[ -z "$compose_override" ]]; then
            compose_override+="
services:
  gitlab:
    deploy:
      resources:
        limits:
          memory: '$2'
"
          else
            compose_override+="          memory: '$2'
"
          fi
          shift 2
        else
          echo "Error: --memory requires a value"
          exit 1
        fi
        ;;
      *)
        echo "Unknown option: $1"
        exit 1
        ;;
    esac
  done

  # Set GitLab version
  export GITLAB_VERSION="$gitlab_version"

  # Create override file if needed
  local compose_files="-f $compose_file"
  if [[ "$use_override" == true ]]; then
    echo "$compose_override" > "$script_dir/docker-compose.override.yml"
    compose_files+=" -f $script_dir/docker-compose.override.yml"
  fi

  cecho b "Starting services with Docker Compose..."
  docker compose $compose_files up -d --build

  # Clean up override file
  if [[ "$use_override" == true ]]; then
    rm -f "$script_dir/docker-compose.override.yml"
  fi

  echo ""
  cecho b "Waiting for GitLab to be healthy..."
  local max_wait=180
  local elapsed=0
  while [ $elapsed -lt $max_wait ]; do
    if docker compose $compose_files ps gitlab 2>/dev/null | grep -q "healthy"; then
      cecho g "✓ GitLab is healthy"
      break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    if [ $((elapsed % 20)) -eq 0 ]; then
      echo "  Still waiting... (${elapsed}s/${max_wait}s)"
    fi
  done
  
  if [ $elapsed -ge $max_wait ]; then
    cecho r "GitLab did not become healthy in time"
    cecho y "Check logs with: ./manage.sh logs-gitlab"
    exit 1
  fi

  echo ""
  cecho b "Configuring GitLab settings..."
  sleep 5  # Brief wait to ensure GitLab API is ready
  curl -s -X PUT -H "PRIVATE-TOKEN: token-string-here123" \
    -d "allow_local_requests_from_web_hooks_and_services=true" \
    -d "allow_local_requests_from_system_hooks=true" \
    -d "outbound_local_requests_allowlist_raw=bot.local" \
    "http://localhost:8080/api/v4/application/settings" > /dev/null && \
    cecho g "✓ GitLab configured to allow local webhooks" || \
    cecho y "⚠ GitLab configuration may have failed (container might still be initializing)"

  # Generate files with GitLab URL and access token
  echo "http://localhost:8080" > "$script_dir/gitlab_url.txt"
  echo "token-string-here123" > "$script_dir/gitlab_token.txt"

  echo ""
  cecho g "=== Test Environment Started Successfully! ==="
  echo ""
  cecho b "GitLab:"
  echo "  • URL: http://localhost:8080"
  echo "  • User: root"
  echo "  • Password: mK9JnG7jwYdFcBNoQ3W3"
  echo "  • API Token: token-string-here123"
  echo ""
  cecho b "Bot:"
  echo "  • Webhook: http://localhost:8081/webhook"
  echo "  • Health: http://localhost:8081/health"
  echo ""
  cecho b "Next steps:"
  cecho y "  make test-integration    # Run integration tests"
  cecho y "  $0 logs -f               # Follow container logs"
  cecho y "  $0 stop                  # Stop environment"
  echo ""
}

stop_environment() {
  cecho b "=== Stopping Test Environment ==="
  echo ""

  docker compose -f "$compose_file" down

  echo ""
  cecho g "✓ Test environment stopped"
  echo ""
}

show_status() {
  cecho b "=== Test Environment Status ==="
  echo ""

  docker compose -f "$compose_file" ps

  echo ""
}

show_logs() {
  local service="${1:-}"
  local extra_args=("${@:2}")

  if [[ -z "$service" ]]; then
    docker compose -f "$compose_file" logs "${extra_args[@]}"
  else
    docker compose -f "$compose_file" logs "$service" "${extra_args[@]}"
  fi
}

# ============================================================================
#  Main Logic
# ============================================================================

if [[ $# -eq 0 ]]; then
  show_usage
  exit 1
fi

command=$1
shift

case $command in
  start)
    start_environment "$@"
    ;;
  stop)
    stop_environment
    ;;
  restart)
    stop_environment
    sleep 2
    start_environment "$@"
    ;;
  status)
    show_status
    ;;
  logs)
    show_logs "" "$@"
    ;;
  logs-gitlab)
    show_logs "gitlab" "$@"
    ;;
  logs-bot)
    show_logs "bot" "$@"
    ;;
  --help|-h|help)
    show_usage
    ;;
  *)
    echo "Unknown command: $command"
    echo ""
    show_usage
    exit 1
    ;;
esac