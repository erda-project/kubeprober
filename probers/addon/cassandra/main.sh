#!/bin/bash

set -o errexit -o nounset -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

function check_cassandra() {
    for i in $(kubectl get pod -n default| grep addon-cassandra | awk '{print $1}')
    do
        if ! (kubectl get pod $i -n default) | tail -n 1 | grep -i running | grep "1/1" >/dev/null 2>/dev/null ; then
          report-status --name=check_cassandra --status=error --message="cassandra_check error node:$i is not running"
          return 1
        fi

        status=$(kubectl  exec -it $i nodetool status -n default| grep rack | awk '{print $1}' | sort | uniq)
        if [[ $status !=  "UN" ]]; then
          report-status --name=check_cassandra --status=error --message="cassandra_check error node:$i have some peer node down"
          return 1
        fi
    done
    report-status --name=check_cassandra --status=pass --message="-"
}

if kubectl get cm dice-cluster-info -n default -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
  check_cassandra
fi