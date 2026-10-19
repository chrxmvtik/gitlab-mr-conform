#!/usr/bin/env bash

# ============================================================================
#  GitLab Deployment Script
#
#  This script pulls the specified GitLab Docker image, stops and removes any
#  existing GitLab container, configures the necessary volumes and environment,
#  and starts a new GitLab container with predefined settings.
#
#  It also generates files with the GitLab URL and access token for use in
#  other scripts or tests.
# ============================================================================

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

# ============================================================================
#  Defaults and Argument Parsing
# ============================================================================

# Default values
gitlab_version="latest"
gitlab_flavor="ce"
cpu_limit=""
memory_limit=""

# Process command-line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "  --help                  Display this help message."
      echo "  --gitlab-version <version> Specify the GitLab version to deploy (default: latest)."
      echo "  --cpus <value>          Limit CPU usage (e.g., 0.5, 2, etc.). Default: no limit."
      echo "  --memory <value>        Limit memory usage (e.g., 512m, 2g, etc.). Default: no limit."
      exit 0
      ;;
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
        cpu_limit="$2"
        shift 2
      else
        echo "Error: --cpus requires a value"
        exit 1
      fi
      ;;
    --memory)
      if [[ -n "$2" ]]; then
        memory_limit="$2"
        shift 2
      else
        echo "Error: --memory requires a value"
        exit 1
      fi
      ;;
    --*)
      echo "Unknown option: $1"
      exit 1
      ;;
    *)
      echo "Unknown argument: $1"
      exit 1
      ;;
  esac
done

# ============================================================================
#  Variables
# ============================================================================

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
repo_root_dir=$(cd "$script_dir/../.." &>/dev/null && pwd)

# Determine the appropriate GitLab image
gitlab_image="gitlab/gitlab-$gitlab_flavor"

# Prepare extra Docker run options
extra_options=""
if [[ -n "$cpu_limit" ]]; then
  extra_options+=" --cpus $cpu_limit"
fi
if [[ -n "$memory_limit" ]]; then
  extra_options+=" --memory $memory_limit"
fi

container_name="mr-conform-gitlab"

# ============================================================================
#  Main Logic
# ============================================================================

cecho b "Checking for existing GitLab container..."
# Check for existing container and skip if found
existing_container_id=$(docker ps -a -f "name=$container_name" --format "{{.ID}}")
if [[ -n $existing_container_id ]]; then
  cecho y "GitLab container '$container_name' already exists. Skipping creation."
  cecho y "To recreate it, please stop and remove the existing container first."
  exit 0
fi

# Pull the GitLab Docker image
cecho b "Pulling GitLab image version '$gitlab_version'..."
docker pull "$gitlab_image:$gitlab_version"

# Start a new GitLab container
cecho b "Starting GitLab..."
docker run --detach \
  --hostname localhost \
  --network host \
  --name "$container_name" \
  --restart always \
  --volume "$repo_root_dir/test/docker/healthcheck-and-setup.sh:/healthcheck-and-setup.sh" \
  --volume "$repo_root_dir/test/docker/gitlab.rb:/etc/gitlab/gitlab.rb:ro" \
  $extra_options \
  --health-cmd '/healthcheck-and-setup.sh' \
  --health-interval 2s \
  --health-timeout 2m \
  "$gitlab_image:$gitlab_version"

cecho b "Waiting 2 minutes before checking if GitLab has started..."
cecho b "(Run this in another terminal to follow the instance logs:"
cecho y "docker logs -f ${container_name}"
cecho b ")"
sleep 120

"$script_dir/await-healthy.sh"

# Generate files with GitLab URL and access token
echo "http://localhost" > "$repo_root_dir/test/docker/gitlab_url.txt"
echo "token-string-here123" > "$repo_root_dir/test/docker/gitlab_token.txt"

# Display GitLab information
cecho b 'GitLab started successfully!'
echo ''
cecho b 'GitLab version:'
curl -s -H "Authorization:Bearer $(cat "$repo_root_dir/test/docker/gitlab_token.txt")" http://localhost/api/v4/version
echo ''

cecho b 'GitLab web UI URL (user: root, password: mK9JnG7jwYdFcBNoQ3W3 )'
echo 'http://localhost'
echo ''

# Provide instructions for stopping and starting GitLab
cecho b 'To stop and delete the GitLab container, run:'
cecho r "make gitlab-stop"
echo ''

cecho b 'To start GitLab container again, re-run this script.'
cecho b 'Note: GitLab will NOT keep any data, so the start will take time.'
cecho b '(This is the only way to make GitLab in Docker stable.)'
echo ''

cecho b 'To start the integration tests, run:'
cecho y 'make test-integration'
echo ''