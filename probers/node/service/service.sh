#!/bin/bash

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
        echo host_image warn "docker image number should no more than 200"
    else
        echo host_image ok
    fi
}


function check_docker_dir() {
    docker_data_dir=$(cat /netdata/dice-ops/dice-config/config.yaml  | grep data_root: | grep -v "#" | awk -F":" '{print $2}' | sed 's/^\s*\|\s*$//g')
    docker_data_dir=${docker_data_dir:="/data/docker/data"}

    dataroot=$(docker info -f '{{.DockerRootDir}}')
    if [[ $dataroot != $docker_data_dir ]]; then
        echo host_dockerdir error "docker data-root should be '$docker_data_dir'"
        return
    fi
    echo host_dockerdir ok
}

function check_kubelet_status() {
    if systemctl is-active kubelet | grep active > /dev/null 2>&1; then
        echo host_kubeletstatus ok
    else
        echo host_kubeletstatus error "kubelet not running"
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
    	[[ ! $config_string =~ $images_string  ]] && echo -n "" "imagefs.available is not 18%"
	    [[ ! $config_string =~ $memory_string  ]] && echo -n "" "memory.available is not 512M"
    	[[ ! $config_string =~ $nodefs_string  ]] && echo -n "" "nodefs.available is not 5%"
	    [[ ! $config_string =~ $nodefs_string2 ]] && echo -n "" "nodefs.inodesFree is not 5%"
    	echo ""
    fi
}

check_docker_status
check_container_number
check_image_number
check_docker_dir
check_kubelet_status
check_firewall
check_resolved
check_ntpd
check_docker_notify
check_kubelet_eviction_config
