#!/bin/bash

set -o errexit -o nounset -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

function sential_detect() {
    ## redis sential ready pod detect
    REPLICAS=$(kubectl get deploy rfs-addon-redis -o yaml | grep ' replicas' | awk '{if (NR > 1){print $2}}')
    READY=$(kubectl get deploy rfs-addon-redis -o yaml | grep readyReplicas | awk '{print $2}')
    if [ "$REPLICAS" != "$READY" ]; then
      report-status --name=check_redis_sential --status=error --message="redis sentail ready pod not statisfied expect"
      return 1
    else
      ## redis sentail connect detect
      kubectl get pods -o wide | grep rfs-addon-redis | awk '{print $6}' | while read i
      do
        if ! nc -z -w 1 "$i" "$S_PORT">/dev/null 2>/dev/null; then
          report-status --name=check_redis_sential --status=error  --message="redis_sential_check error redis-sentinal:$i $S_PORT connect failed"
          return 1
        fi
      done
      report-status --name=check_redis_sential --status=ok
    fi
}

function redis_detect() {
    ## redis ready pod detect
    REPLICAS=$(kubectl get sts rfr-addon-redis -o yaml | grep ' replicas' | awk '{if (NR > 1){print $2}}')
    READY=$(kubectl get sts rfr-addon-redis -o yaml | grep readyReplicas | awk '{print $2}')
    if [ "$REPLICAS" != "$READY" ]; then
        report-status --name=check_redis --status=error --message="redis ready pod not statisfied expect"
        return 1
    else
      MASTER_INFO=$(echo info | nc "$S_IP" "$S_PORT"  2>/dev/null| grep master0)
      if [ $? != 0 ];then
          report-status --name=check_redis --status=error --message="get redis info failed"
          return 1
      else
          ## redis connect detect
          MASTER_INFO=${MASTER_INFO#*address=}
          MASTER_INFO=${MASTER_INFO%,slave*}
          R_IP=${MASTER_INFO%:*}
          R_PORT=${MASTER_INFO#*:}
          kubectl get pods -o wide | grep rfr-addon-redis | awk '{print $6}' | while read i
          do
            if ! nc -z -w 1 "$i" "$R_PORT">/dev/null 2>/dev/null; then
              report-status --name=check_redis --status=error --message="redis:$i $R_PORT connect failed"
              return 1
            fi
          done

          if ! ((printf "AUTH $R_PASSWORD\r\n";  echo set probe_test_key v1) | nc $R_IP $R_PORT) | tail -n 1 | grep OK >/dev/null 2>/dev/null ; then
            report-status --name=check_redis  --status=error --message="redis:$R_IP write key error"
            return 1
          fi

          if ! ((printf "AUTH $R_PASSWORD\r\n";  echo del probe_test_key) | nc $R_IP $R_PORT) | tail -n 1 | grep ":1"  >/dev/null 2>/dev/null; then
            report-status --name=check_redis --status=error --message="redis:$R_IP delete key error"
            return 1
          fi

          ## redis master slave detect
          kubectl get pods -o wide | grep rfr-addon-redis | grep -v "$R_IP" | awk '{print $6}' | while read i
          do
            if ! ((printf "AUTH $R_PASSWORD\r\n";  echo info replication) | nc $i  "$R_PORT" 2>/dev/null) | grep "master_link_status:up">/dev/null 2>/dev/null; then
              report-status --name=check_redis --status=error  --message="redice slave:$i psync failed"
              return 1
            fi
          done
      fi
      report-status --name=check_redis --status=ok --message="-"
    fi
}

## 中央集群执行 redis 检测
if kubectl get cm dice-cluster-info -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
	export S_PORT=$(kubectl get svc rfs-addon-redis -o yaml | grep ' targetPort:' | awk '{print $2}')
  export S_IP=$(kubectl get svc rfs-addon-redis -o yaml | grep clusterIP | awk '{print $2}')
  export R_PASSWORD=$(kubectl get cm dice-addons-info -o yaml | grep ' REDIS_PASSWORD' | awk '{print $2}')
  export REDIS_EXISTS=true
	if [ "$REDIS_EXISTS" == "true" ]; then
		sential_detect
		redis_detect
	fi
fi