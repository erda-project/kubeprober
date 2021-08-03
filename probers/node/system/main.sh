#!/bin/bash

cluster_dns=$(kubectl get svc -n kube-system | grep kube-dns | awk '{print $3}')
cluster_dns=${cluster_dns:="-"}
kubectl probe node /checkers/system/system.sh $cluster_dns