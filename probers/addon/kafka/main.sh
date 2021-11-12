#!/bin/bash
set -o errexit -o nounset -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# env:
# KAFKA_LAG_MAX is the alert threshold
IFS=" "
function check_kafka_topic_lag() {
  LAG_ALERT_MSG=""
  pod=$(kubectl get pod -n default| grep addon-kafka | grep -v connect | grep -i running | grep "1/1" | awk '{print $1}' | head -n 1)

  while read -r GROUP; do
    while read -r line; do
      if [ -z "$line" ];then
        continue
      fi
      read -ra ADDR <<< "$line"
      topic=${ADDR[0]}
      lag=${ADDR[1]}
      if [ $lag -gt $KAFKA_LAG_MAX ]; then
        LAG_ALERT_MSG+="topic=$topic, consumer-group=$GROUP, lag=$lag\n"
        echo $LAG_ALERT_MSG
      fi
    done <<< $(kubectl exec "$pod" -n default -- bash -c "unset JMX_PORT && /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group $GROUP" | sed '1,2d;$d' | sort | awk '{sum[$1]+=$5} END {for (val in sum) print val, sum[val]}')
  done <<< $(kubectl exec "$pod" -n default -- bash -c 'unset JMX_PORT && /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092  --list' |grep -e spot- -e msp- | sort | uniq )

  if [ -n "$LAG_ALERT_MSG" ]; then
    #echo "kafka lag alert:\n$LAG_ALERT_MSG"
    report-status --name=kafka_lag --status=error --message="kafka lag alert:\n$LAG_ALERT_MSG"
  else
    report-status --name=kafka_lag --status=pass --message="-"
  fi
}


if kubectl get cm dice-cluster-info -n default -o yaml | grep DICE_IS_EDGE: | grep false>/dev/null 2>/dev/null; then
  check_kafka_topic_lag
fi