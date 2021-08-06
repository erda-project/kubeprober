#!/bin/bash

function check_glusterfs() {
  STORAGE_TYPE=$(kubectl get cm dice-tools-info -o 'jsonpath={.data.STORAGE_TYPE}' 2>/dev/null)
  STORAGE_SERVERS=$(kubectl get cm dice-tools-info -o 'jsonpath={.data.STORAGE_SERVERS}' 2>/dev/null)
  STORAGE_REMOTE_TARGET=$(kubectl get cm dice-tools-info -o 'jsonpath={.data.STORAGE_REMOTE_TARGET}' 2>/dev/null)

  if [[ "$STORAGE_TYPE" != glusterfs ]]; then
    return 1
  fi

  if ! command -v gluster >/dev/null 2>&1; then
    yum install -y glusterfs-cli
  fi

  rh=$(echo "$STORAGE_REMOTE_TARGET" | awk -F: '{print $1}')
  dv=$(echo "$STORAGE_REMOTE_TARGET" | awk -F: '{print $2}')

  s=$(gluster --remote-host="$rh" volume status "$dv" detail)
  n=$(echo "$STORAGE_SERVERS" | tr ',' '\n' | wc -l)
  y=$(echo "$s" | grep '^Online' | grep 'Y\s*$' | wc -l)
  if [[ "$n" -eq "$y" && "$(echo "$s" | grep '^Online' | wc -l)" -eq "$y" ]]; then
    report-status --name=check_glusterfs ok "-"
  else
    report-status --name=check_glusterfs error "online expect $n buy $y"
  fi
}

check_glusterfs