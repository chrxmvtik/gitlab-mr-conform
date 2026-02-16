#!/usr/bin/env sh

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.


printf 'Waiting for GitLab container to become healthy'
container_name="mr-conform-gitlab"
until test -n "$(docker ps --quiet --filter name=$container_name --filter health=healthy)"; do
  printf '.'
  sleep 5
done

echo
echo 'GitLab is healthy'
