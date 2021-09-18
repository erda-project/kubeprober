#!/bin/bash

cluster_dns=$1

# 单机检查脚本
# 每个子检查脚本的输出规范：
# 检测通过              echo 检查项名 ok
# 检测不通过            echo 检查项名 error 错误信息
# 检查不通过但可以容忍    echo 检查项名 info 错误信息

# 检查 /etc/resolv.conf 是否存在问题
function check_resolv_file() {

    ## resolve.conf exists judge
    if [ ! -f /etc/resolv.conf ]; then
        echo "host_resolv_file" "error" "/etc/resolv.conf dose not exists"
        return
    fi

    ## ali linux does not judge attr lock
    if grep 'Alibaba' /etc/redhat-release > /dev/null ; then
        echo "host_resolv_file" "ok"
        return
    fi

    ## judge lock resolve.conf or not
    check_info=$(lsattr /etc/resolv.conf)
    result=$(echo $check_info | grep "i")
    if [[ "$result" != "" ]]
    then
        echo "host_resolv_file" "ok"
    else
        echo "host_resolv_file" "error" "/etc/resolv.conf have not locked"
    fi
}

# 检查dns配置
function check_dns() {
    if [[ $cluster_dns == "-" ]]; then
        return
    fi

    ## resolve.conf exists judge
    if [ ! -f /etc/resolv.conf ]; then
        echo "host_dns" "error" "/etc/resolv.conf dose not exists"
        return
    fi

    result=$(cat /etc/resolv.conf | grep $cluster_dns)
    if [[ "$result" != "" ]]
    then
        echo "host_dns" "ok"
    else
        echo "host_dns" "error" "host dns config is not cluster dns ip，current dns [$cluster_dns]"
    fi
}

# 检查内存碎片
function check_buddy_info() {
    compact_flag=$(cat /proc/buddyinfo | grep "Normal" | awk '{
        if($NF == "0" && $(NF-1) == "0" && $(NF-2) == "0" && $(NF-3) == "0" && $(NF-4) == "0"){
            print "ok"
        } else {
            print "notok"
        }
    }')
    if [[ $compact_flag == "ok" ]]
    then
        echo "host_buddy_info" "warn" "buddy_info need to be compact"
    else
        echo "host_buddy_info" "ok"
    fi
}

# 检查内核参数
function check_kernel_param() {
    kernel=$(uname -r | egrep -m 1 -o '[0-9]+(\.[0-9]+)+' | head -n 1)
    if echo "$kernel" | grep -E "^5.*|^4.*" >/dev/null 2>/dev/null; then
        check_list=("net.bridge.bridge-nf-call-ip6tables=1" \
                "net.bridge.bridge-nf-call-iptables=1" \
                "net.ipv4.ip_forward=1" \
                "net.ipv4.tcp_keepalive_time=600" \
                "net.ipv4.tcp_timestamps=0" \
                "net.ipv4.tcp_max_syn_backlog=4096" \
                "net.core.somaxconn=4096" \
                "vm.max_map_count=262144" \
                "vm.overcommit_memory=1" \
                "vm.swappiness=0" \
                "net.netfilter.nf_conntrack_tcp_timeout_established=300" \
                "net.netfilter.nf_conntrack_tcp_timeout_time_wait=30" \
                "net.netfilter.nf_conntrack_tcp_timeout_fin_wait=30" \
                "net.netfilter.nf_conntrack_tcp_timeout_close_wait=15" \
                "net.ipv4.neigh.default.gc_thresh1=1024" \
                "net.ipv4.neigh.default.gc_thresh2=2048" \
                "net.ipv4.neigh.default.gc_thresh3=4096" \
                "fs.inotify.max_user_watches=524288" )
    else
        check_list=("fs.may_detach_mounts=1" \
                "net.bridge.bridge-nf-call-ip6tables=1" \
                "net.bridge.bridge-nf-call-iptables=1" \
                "net.ipv4.ip_forward=1" \
                "net.ipv4.tcp_keepalive_time=600" \
                "net.ipv4.tcp_timestamps=0" \
                "net.ipv4.tcp_max_syn_backlog=4096" \
                "net.core.somaxconn=4096" \
                "vm.max_map_count=262144" \
                "vm.overcommit_memory=1" \
                "vm.swappiness=0" \
                "net.netfilter.nf_conntrack_tcp_timeout_established=300" \
                "net.netfilter.nf_conntrack_tcp_timeout_time_wait=30" \
                "net.netfilter.nf_conntrack_tcp_timeout_fin_wait=30" \
                "net.netfilter.nf_conntrack_tcp_timeout_close_wait=15" \
                "net.ipv4.neigh.default.gc_thresh1=1024" \
                "net.ipv4.neigh.default.gc_thresh2=2048" \
                "net.ipv4.neigh.default.gc_thresh3=4096" \
                "fs.inotify.max_user_watches=524288" )
    fi
    local check_result
    for item in ${check_list[@]}
    do
        key=$(echo $item | awk -F"=" '{print $1}')
        vakue=$(echo $item | awk -F"=" '{print $2}')
        if [[ $(sysctl -n $key 2>/dev/null) != $vakue ]]
        then
            check_result=$check_result","$key
        fi
    done
    # ipvsadm udp timeout 巡检
    if [[ $(ipvsadm -l --timeout|awk '{print $NF}') != "1" ]]
    then
            check_result=$check_result", ipvsadm_udp_timeout"
    fi
    if [[ $check_result == "" ]]
    then
        echo "host_kernel_param" "ok"
    else
        echo "host_kernel_param" "warn" ${check_result:1:65535}" is not correct"
    fi

}


function check_not_k8s_container() {
    # check by container label and create time
    old=$(date +"%Y-%m-%d" -d "-1day")
    if docker ps --format "table {{.Names}}\t{{.CreatedAt}}\t{{.Labels}}"|tail -n +2|grep -iv "io.kubernetes.pod.name"| awk '{if ($2<"'$old'") {print $1 " not_k8s_container"}}'|grep "not_k8s_container" >/dev/null 2>&1; then
        echo host_not_k8s_container warn "host have container is not k8s container"
    else
        echo host_not_k8s_container ok
    fi
}

function check_ipvs_module() {
    module_num=$(lsmod | awk '{print $1}' | grep -Eo "^ip_vs$|^ip_vs_rr$|^ip_vs_sh$|^ip_vs_wrr$"  | wc -l)
    if [[ $module_num == 4 ]]; then
        echo host_ipvs_module ok
    else
        echo host_ipvs_module error "Lack of some ipvs module [ip_vs,ip_vs_rr,ip_vs_sh,ip_vs_wrr]"
    fi
}

function check_zombie_process_num() {
    num=$(ps -ef | grep defunct | grep -v grep | wc -l)
    if [[ $num -gt 1000 ]]; then
        echo host_zombie_process warn "zombie process number more than 500"
    else
        echo host_zombie_process ok
    fi
}

function check_iptables_forward() {
    if /usr/bin/env iptables -nL|grep 'Chain FORWARD (policy ACCEPT)' >/dev/null 2>&1; then
        echo host_iptables_forward ok
    else
        echo host_iptables_forward error "Chain FORWARD (policy ACCEPT) does not exist"
    fi
}
# 节点mem异常是否异常
function check_system_mem() {
    tail -n 5000 /var/log/messages| grep -iE "unable to allocate memory|cannot allocate memory" >>/dev/null 2>&1 && echo "host_system_mem" "warn" "unable to allocate memory|cannot allocate memory" && return
    echo "host_system_mem" "ok"
}

check_resolv_file
check_dns
check_buddy_info
check_kernel_param
check_not_k8s_container
check_ipvs_module
check_zombie_process_num
check_iptables_forward
check_system_mem