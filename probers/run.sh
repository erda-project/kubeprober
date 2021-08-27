#!/bin/bash

#set -o errexit

cd "$(dirname "${BASH_SOURCE[0]}")"

for app in $(find * -maxdepth 0 -type d)
do
  if [ ! -f "$app"/main ]; then
    echo "warning: no main file under path /checkers/$app"
    exit 1
  else
    chmod +x "$app"/main
  fi
  echo "start to run $app"
  /checkers/$app/main
done