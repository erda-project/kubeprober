#!/bin/bash
cd "$(dirname "${BASH_SOURCE[0]}")"

function check_elasticsearch() {
  IFS_old=$IFS
  IFS=$'\n'
  for i in $(kubectl  get svc | grep elastic | grep 9200)
  do
    name=$(echo $i | awk '{print $1}')
    ip=$(echo $i | awk '{print $3}')

    # query es status
    status=$(curl -s $ip:9200/_cat/health?v | tail -n 1 | awk '{print $4}')
    if [[ $status == "red" ]]; then
      report-status --name=check_elasticsearch --status=error --message="$name status is red"
      return 1
    fi

    # create index
    code=$(curl --connect-timeout 3 -sL -w "%{http_code}"  -X PUT "$ip:9200/probe_test_index" -o /tmp/create_es_index_result)
    if [[ $code != 200 ]]
    then
      reason=$(cat /tmp/create_es_index_result | jq '.error.reason')
      report-status --name=check_elasticsearch --status=error --message="create index error: $reason"
      return 1
    fi

    # delete index
    code=$(curl --connect-timeout 3 -sL -w "%{http_code}"  -X DELETE "$ip:9200/probe_test_index" -o /tmp/delete_es_index_result)
    if [[ $code != 200 ]]
    then
      reason=$(cat /tmp/delete_es_index_result | jq '.error.reason')
      report-status --name=check_elasticsearch --status=error --message="delete index error: $reason"
      return 1
    fi

  done
  IFS=$IFS_old
  report-status --name=check_elasticsearch --status=ok --message="-"
}

if kubectl get cm dice-cluster-info -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
  check_elasticsearch
fi