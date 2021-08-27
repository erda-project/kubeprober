#!/bin/bash
cd "$(dirname "${BASH_SOURCE[0]}")"

function check_zookeeper() {
  IFS_old=$IFS
  IFS=$'\n'
  for i in $(kubectl  get svc -n default | grep zookeeper | grep 2181)
  do
    name=$(echo $i | awk '{print $1}')
    ip=$(echo $i | awk '{print $3}')

    # query es status
    status=$(echo ruok | nc $ip 2181)
    if [[ $status != "imok" ]]; then
      report-status --name=check_zookeeper --status=error --message="$name ruok status is not imok"
      return 1
    fi

  done
  IFS=$IFS_old
  report-status --name=check_zookeeper --status=pass --message="-"
}

if kubectl get cm dice-cluster-info -n default -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
  check_zookeeper
fi