#!/usr/bin/env bash

container_name="mr-conform-bot"

existing_container_id=$(docker ps -a -f "name=$container_name" --format "{{.ID}}")
if [[ -n $existing_container_id ]]; then
  echo "Stopping and removing existing MR Conform bot container..."
  docker stop --timeout=30 "$existing_container_id"
  docker rm "$existing_container_id"
fi