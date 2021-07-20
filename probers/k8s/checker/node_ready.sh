#!/bin/bash

ErrorStatus = "ERROR"
WarnStatus = "WARN"
UnknowStatus = "UNKNOWN"
InfoStatus = "INFO"
PassStatus = "PASS"


function report_status() {
  report_status --name=$1 --status=$2 --message=$3
}

function check_node_ready() {
    CHECKER_NAME="node_ready"
    if kubectl get node -o jsonpath='{range .items[*].status}{range .conditions[?(@.type=="Ready")]}{.status}{"\n"}{end}{end}' | grep False > /dev/null 2>&1; then
        report_status $CHECKER_NAME ErrorStatus "NotReady node found"
    else
        report_status $CHECKER_NAME $PassStatus
    fi
}

check_node_ready