#!/bin/bash -x
kubectl logs --since=5m  ds/nginx-ingress-controller  -n kube-system  | awk '
  {
    if($(NF-19) == "openapi.erda.cloud" || $(NF-19) == "kubeprober.erda.cloud")
    {
      num[$(NF-19)]++
      sum[$(NF-19)]+=$(NF-4)
    }
  }
  END{
    for (i in num) {
      print i, "  ", sum[i]/num[i]
    }
  }
' > /tmp/rt_result

IFS_old=$IFS
IFS=$'\n'

timestamp=$(date +%s)000000000
for i in $(cat /tmp/rt_result)
do
    domain=$(echo $i | awk '{print $1}')
    rt=$(echo $i | awk '{print $2}')
    curl --request POST "$INFLUXADDR" --header "Authorization: Token $TOKEN" --data-binary "rt,domain=$domain rt=$rt $timestamp"
done