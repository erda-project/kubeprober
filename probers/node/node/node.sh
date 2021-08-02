#!/bin/bash

if [[ $1 == "-" ]]
then
    docker_data_dir='/data/docker/data'
else
    docker_data_dir=$1
fi

cluster_dns=$2
is_cs=$3

# 单机检查脚本
# 每个子检查脚本的输出规范：
# 检测通过              echo 检查项名 ok
# 检测不通过            echo 检查项名 error 错误信息
# 检查不通过但可以容忍    echo 检查项名 info 错误信息

# TODO: _netdev参数是否添加
# 检查数据盘，网盘的挂载情况
function check_mount_info() {
    datadisk=$(df -T 2>/dev/null | grep "/data" | grep "/dev")
    netdisk=$(df -T 2>/dev/null| grep "/netdata$" | grep -E "nfs|glusterfs|cifs")
    if [[ -z "$datadisk" && "$is_cs" == false ]]; then
        if [[ $(lsblk -b |egrep "/$"|awk '{print $(NF-3)}'|uniq) < 150000000000 ]]; then
            echo "host_mount_info" "error" "data disk not mount"
            return
        fi
    fi
    if [[ "$netdisk" == "" ]]; then
        echo "host_mount_info" "error" "netdata disk not mount"
        return
    fi
    netdev=$(cat /etc/fstab | grep netdata | grep "_netdev")
    if [[ $netdev == "" ]]
    then
        echo "host_mount_info" "error" "/netdata mount without _netdev param"
        return
    fi
    cd /netdata && ls 1>/dev/null 2>/dev/null
    if [[ "$?" != 0 ]]
    then
        echo "host_mount_info" "error" "can not work on /netdata"
        return
    fi
    echo "host_mount_info" "ok"
}

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
        echo "host_dns" "error" "host dns config is not cluster dns ip"
    fi
}
# 检查dice账户有没有创建?
function check_dice_user() {
    id dice 2>/dev/null 1>/dev/null
    if [[ $? == 0 ]]
    then
        echo "host_dice_user" "ok"
    else
        echo "host_dice_user" "error" "dice account not set"
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
        echo "host_buddy_info" "error" "buddy_info need to be compact"
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
        echo "host_kernel_param" "error" ${check_result:1:65535}" is not correct"
    fi

}

## docker层面检查
function check_docker_status() {
    if systemctl is-active docker | grep active > /dev/null 2>&1; then
        echo host_dockerstatus ok
    else
        echo host_dockerstatus error "docker not running"
    fi
}

function check_container_number() {
    num=$(docker info -f '{{.Containers}}')
    if [[ $num -gt 200 ]]; then
        echo host_container info "docker container(with exited) number should no more than 200"
    else
        echo host_container ok
    fi
}

function check_image_number() {
    num=$(docker info -f '{{.Images}}')
    if [[ $num -gt 200 ]]; then
        echo host_image info "docker image number should no more than 200"
    else
        echo host_image ok
    fi
}



function check_not_k8s_container() {
    # check by container label and create time
    old=$(date +"%Y-%m-%d" -d "-1day")
    if docker ps --format "table {{.Names}}\t{{.CreatedAt}}\t{{.Labels}}"|tail -n +2|grep -iv "io.kubernetes.pod.name"| awk '{if ($2<"'$old'") {print $1 " not_k8s_container"}}'|grep "not_k8s_container" >/dev/null 2>&1; then
        echo host_not_k8s_container error "host have container is not k8s container"
    else
        echo host_not_k8s_container ok
    fi
}

function check_docker_version() {
    clientversion=$(docker version -f '{{.Client.Version}}')
    servervesion=$(docker version -f '{{.Server.Version}}')
    if [[ $clientversion != '18.09.5' || $servervesion != '18.09.5' ]]; then
        echo host_dockerversion info "docker version should be 18.09.5"
    else
        echo host_dockerversion ok
    fi
}

#TODO: EXEC DIR
function check_docker_dir() {
    # systemctl status docker | grep '\--config-file=/etc/docker/daemon.json' > /dev/null 2>&1
    # if [[ $? == 1 ]]; then
    #     echo host_dockerdir error "docker dir maybe not configed"
    #     return
    # fi

    dataroot=$(docker info -f '{{.DockerRootDir}}')
    if [[ $dataroot != $docker_data_dir ]]; then
        echo host_dockerdir error "docker data-root should be '/data/docker/data'"
        return
    fi
    echo host_dockerdir ok
}

function check_kubelet_version() {
    version=$(kubelet --version)
    if [[ $version == 'Kubernetes v1.13.10' || $version == 'Kubernetes v1.16.4' ]]; then
        echo host_kubeletversion ok
    else
        echo host_kubeletversion info "kubelet version should be v1.13.10"
    fi
    ## get kubelet version info
    echo "node_kubelet_version $(kubelet --version | awk '{print $2}')"
}

function check_kubelet_status() {
    if systemctl is-active kubelet | grep active > /dev/null 2>&1; then
        echo host_kubeletstatus ok
    else
        echo host_kubeletstatus error "kubelet not running"
    fi
}

function check_kubelet_datadir() {
    if ps aux | grep kubelet | grep '\--root-dir=/data/kubelet' > /dev/null 2>&1; then
        echo host_kubeletdatadir ok
    else
        echo host_kubeletdatadir error "kubelet datadir maybe not configed"
    fi
}

function check_firewall() {
    if systemctl is-active firewalld >/dev/null 2>/dev/null; then
        echo host_firewall error "firewall should be disabled but not"
    else
        echo host_firewall ok
    fi
}

function check_resolved() {
    source /etc/os-release
    if [[ "$VERSION_ID" == '8' ]]; then
        if systemctl is-active systemd-resolved >/dev/null 2>/dev/null; then
            echo host_resolved error "resolved should be disabled but not"
        else
            echo host_resolved ok
        fi
    fi
}

function check_ntpd() {
    if systemctl is-active ntpd | grep active > /dev/null 2>&1; then
        echo host_ntpd ok
    else
        echo host_ntpd error "ntpd not running"
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
        echo host_zombie_process error "zombie process number more than 500"
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

function check_docker_notify() {
    if cat /etc/systemd/system/docker.service |grep 'Type=notify' >/dev/null 2>&1; then
        echo docker_service_notify ok
    else
        echo docker_service_notify error "docker service is not Type=notify"
    fi
}

# 检查自建dice vendor集群机器kubelet驱逐pod配置是否合理
function check_kubelet_eviction_config() {
    #判断是否为dice vendor集群
    export DICE_CONFIG=/netdata/dice-ops/dice-config/config.yaml
    export DICE=false

    if [ -f /netdata/dice-ops/dice-config/config.yaml ]; then
        ## 老集群
        if ! cat "$DICE_CONFIG" | grep vendor > /dev/null 2>/dev/null; then
            if ! cat "$DICE_CONFIG" | grep "is_aliyun_k8s" >/dev/null 2>/dev/null; then
                export DICE=true
            else
                if ! cat "$DICE_CONFIG" | grep "is_aliyun_k8s:" | grep true >/dev/null 2>/dev/null; then
                export DICE=true
                fi
            fi
        else
            if cat "$DICE_CONFIG" | grep vendor| grep dice >/dev/null 2>/dev/null; then
                export DICE=true
            fi
        fi
    fi

    if [ "$DICE" == false ]; then
        return
    fi

    config_string=`cat /var/lib/kubelet/config.yaml`
    images_string='imagefs.available: 10%'
    memory_string='memory.available: 512Mi'
    nodefs_string='nodefs.available: 5%'
    nodefs_string2='nodefs.inodesFree: 5%'

    echo -n "host_kubelet_eviction_config"
    if [[ $config_string =~ $images_string && $config_string =~ $memory_string && $config_string =~ $nodefs_string  && $config_string =~ $nodefs_string2 ]] ; then
	    echo "" "ok"
    else
	    echo -n "" "error"
    	[[ ! $config_string =~ $images_string  ]] && echo -n "" "imagefs.available"
	    [[ ! $config_string =~ $memory_string  ]] && echo -n "" "memory.available"
    	[[ ! $config_string =~ $nodefs_string  ]] && echo -n "" "nodefs.available"
	    [[ ! $config_string =~ $nodefs_string2 ]] && echo -n "" "nodefs.inodesFree"
    	echo ""
    fi
}

# 节点mem异常是否异常
function check_system_mem() {
    tail -n 5000 /var/log/messages| grep -iE "unable to allocate memory|cannot allocate memory" >>/dev/null 2>&1 && echo "host_system_mem" "error" "unable to allocate memory|cannot allocate memory" && return
    echo "host_system_mem" "ok"
}

# check_mount_info
check_resolv_file
check_dice_user
check_buddy_info
check_kernel_param
check_docker_status
check_container_number
check_image_number
check_not_k8s_container
check_docker_version
# check_docker_dir
check_kubelet_version
check_kubelet_status
check_kubelet_datadir
check_firewall
check_resolved
# check_dns
check_ntpd
check_ipvs_module
check_zombie_process_num
check_iptables_forward
check_docker_notify
check_kubelet_eviction_config
check_system_mem