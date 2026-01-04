#!/usr/bin/env bash

# ============================================================================
#  GitLab MR Conform Test Environment Manager
#
#  This script orchestrates the test environment by calling the individual
#  scripts for GitLab and bot management.
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
  echo ""
  echo "Options for 'start':"
  echo "  --gitlab-version <version>  Specify GitLab version (default: latest)"
  echo "  --cpus <value>              Limit CPU for GitLab (e.g., 0.5, 2)"
  echo "  --memory <value>            Limit memory for GitLab (e.g., 512m, 2g)"
  echo ""
  echo "Examples:"
  echo "  $0 start"
  echo "  $0 start --gitlab-version 16.0.0"
  echo "  $0 stop"
  echo "  $0 status"
}

# ============================================================================
#  Variables
# ============================================================================

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
repo_root_dir=$(cd "$script_dir/../.." &>/dev/null && pwd)

gitlab_container="gitlab-mr-conform-test"
bot_container="gitlab-mr-conformity-bot"

# ============================================================================
#  Command Functions
# ============================================================================

start_environment() {
  cecho b "=== Starting Test Environment ==="
  echo ""

  # Start GitLab using the dedicated script
  cecho b "Starting GitLab..."
  "$script_dir/run_gitlab.sh" "$@"
  
  echo ""
  
  # Configure GitLab settings via API to allow local webhook requests
  cecho b "Configuring GitLab settings..."
  sleep 2  # Brief wait to ensure GitLab API is ready
  curl -s -X PUT -H "PRIVATE-TOKEN: token-string-here123" \
    -d "allow_local_requests_from_web_hooks_and_services=true" \
    -d "allow_local_requests_from_system_hooks=true" \
    "http://localhost/api/v4/application/settings" > /dev/null
  cecho g "✓ GitLab configured to allow local webhooks"
  
  echo ""
  
  # Start bot using the dedicated script
  cecho b "Starting Bot..."
  "$script_dir/run_bot.sh"
  
  echo ""
  cecho g "=== Test Environment Started Successfully! ==="
  echo ""
  cecho b "GitLab:"
  echo "  • URL: http://localhost"
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
  cecho y "  $0 logs                  # View container logs"
  cecho y "  $0 stop                  # Stop environment"
  echo ""
}

stop_environment() {
  cecho b "=== Stopping Test Environment ==="
  echo ""

  # Stop bot using the dedicated script
  cecho b "Stopping Bot..."
  "$script_dir/stop_bot.sh"
  
  echo ""
  
  # Stop GitLab using the dedicated script
  cecho b "Stopping GitLab..."
  "$script_dir/stop_gitlab.sh"

  echo ""
  cecho g "✓ Test environment stopped"
  echo ""
}

show_status() {
  cecho b "=== Test Environment Status ==="
  echo ""

  # Check GitLab
  if docker ps --filter "name=$gitlab_container" --format "{{.Names}}" | grep -q "$gitlab_container"; then
    local health=$(docker inspect --format='{{.State.Health.Status}}' "$gitlab_container" 2>/dev/null || echo "no-healthcheck")
    if [[ "$health" == "healthy" ]]; then
      cecho g "GitLab: running (healthy)"
    else
      cecho y "GitLab: running ($health)"
    fi
  elif docker ps -a --filter "name=$gitlab_container" --format "{{.Names}}" | grep -q "$gitlab_container"; then
    cecho r "GitLab: stopped"
  else
    cecho r "GitLab: not found"
  fi

  # Check Bot
  if docker ps --filter "name=$bot_container" --format "{{.Names}}" | grep -q "$bot_container"; then
    local health=$(docker inspect --format='{{.State.Health.Status}}' "$bot_container" 2>/dev/null || echo "no-healthcheck")
    if [[ "$health" == "healthy" ]]; then
      cecho g "Bot: running (healthy)"
    else
      cecho y "Bot: running ($health)"
    fi
  elif docker ps -a --filter "name=$bot_container" --format "{{.Names}}" | grep -q "$bot_container"; then
    cecho r "Bot: stopped"
  else
    cecho r "Bot: not found"
  fi

  echo ""
}

show_logs() {
  cecho b "=== Container Logs ==="
  echo ""
  cecho y "GitLab logs: docker logs -f $gitlab_container"
  cecho y "Bot logs: docker logs -f $bot_container"
  echo ""
  
  # Show last 20 lines from each if containers exist
  if docker ps --filter "name=$gitlab_container" --format "{{.Names}}" | grep -q "$gitlab_container"; then
    cecho b "--- GitLab (last 20 lines) ---"
    docker logs --tail 20 "$gitlab_container" 2>&1
    echo ""
  fi

  if docker ps --filter "name=$bot_container" --format "{{.Names}}" | grep -q "$bot_container"; then
    cecho b "--- Bot (last 20 lines) ---"
    docker logs --tail 20 "$bot_container" 2>&1
    echo ""
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
    show_logs
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
