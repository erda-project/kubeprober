#!/bin/bash

# 检查kubedb证书过期时间
function check_kubedb_crt() {
    cname="kubedb_crt_time"
    status="pass"
    msg="-"
    if  kubectl get secret kubedb-operator-apiserver-cert >>/dev/null 2>&1 ; then
        time1=$(kubectl get secret kubedb-operator-apiserver-cert  -o yaml|egrep '^  tls.crt'|awk  '{print $2}'|base64 -d|openssl x509  -noout -dates|grep notAfter|awk -F "=" '{print $2}'|xargs -I {} date -d {} +%s)
        if [ $time1 -ge $(date +%s) ]; then
            if [ $(( ($time1 - $(date +%s))/86400 )) -gt 365 ]; then
                report-status --name="$cname" --status="$status" --message="$msg"
            else
                status=error
                msg="kubedb crt time last $((($time1 - $(date +%s))/86400)) days"
                report-status --name="$cname" --status="$status" --message="$msg"
            fi
        else
            status=error
            msg="kubedb crt time is expired"
            echo "$cnam" "$status" "$msg"
            report-status --name="$cname" --status="$status" --message="$msg"
        fi
    else
        report-status --name="$cname" --status="$status" --message="$msg"
    fi
}

check_kubedb_crt