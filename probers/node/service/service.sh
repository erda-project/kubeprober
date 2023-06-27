#!/bin/bash

cluster_vendor=$(cat /netdata/dice-ops/dice-config/config.yaml | grep vendor | awk '{print $2}' 2>/dev/null)
is_cs=false
if [[ "$cluster_vendor" == cs || "$cluster_vendor" == cs_managed || "$cluster_vendor" == edas ]]; then
    is_cs=true
fi
if docker info  >/dev/null 2>&1; then
    cri_name=docker
else
    cri_name=containerd
fi

function check_cri_status() {
    if systemctl is-active $cri_name | grep '^active' > /dev/null 2>&1; then
        echo "host_$cri_name status ok"
    else
        echo "host_$cri_name status error $cri_name not running"
    fi
}

function check_data_dir() {
    if [[ "$cri_name" == "docker" ]]; then
      if [[ "$is_cs" == true ]]; then
        docker_data_dir="/var/lib/docker"
      else
        docker_data_dir="/data/docker/data"
      fi
      dataroot=$(docker info -f '{{.DockerRootDir}}')
      if [[ $dataroot != $docker_data_dir ]]; then
        if [[ "$cluster_vendor" == cs_managed && $dataroot == '/var/lib/docker' && $docker_data_dir == '/var/lib/container/docker' ]]; then
        # cs_managed ack bind /var/lib/container/docker /var/lib/docker in /etc/fstab
           echo host_dockerdir ok
           return
        fi
        echo host_dockerdir error "docker data-root should be '$docker_data_dir'"
        return
      fi
      echo host_dockerdir ok
      else
        containerd_data_dir=$(cat /netdata/dice-ops/dice-config/config.yaml  | grep state_path:|awk -F":" '{print $2}' |sed 's/^\s*\|\s*$//g')
        containerd_root=$(containerd config dump |tr -d ' '|grep state=|awk -F'"' '{print $2}')
        if [[  "$containerd_data_dir" == "$containerd_root" ]];then
          echo host_containerdir ok
          return
        else
          echo host_containerdir error "container data-root should be '$containerd_data_dir'"
          return
        fi
    fi
}

function check_kubelet_status() {
    if systemctl is-active kubelet | grep '^active' > /dev/null 2>&1; then
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

function check_chronyd() {
    if systemctl is-active chronyd | grep '^active' > /dev/null 2>&1; then
        echo host_chronyd ok
    else
        echo host_chronyd error "chronyd not running"
    fi
}

function check_cri_notify() {
   if grep -q 'Type=notify' "/etc/systemd/system/$cri_name.service" 2>/dev/null ||
   grep -q 'Type=notify' "/etc/systemd/system/multi-user.target.wants/$cri_name.service" 2>/dev/null; then
      echo "$cri_name"_service_notify ok
   else
       echo "$cri_name"_service_notify error: "$cri_name service is not Type=notify"
   fi
}


function check_kubelet_eviction_config() {
  value=$(ps aux | grep /usr/bin/kubelet | egrep -o  "eviction-hard=imagefs.available<([0-9]+)" | awk -F"<" '{print $2}')
  if [ "$value" == "" ]; then
    configFile=$(ps aux | grep "/usr/bin/kubelet" | egrep -o  "\--config=.+" | awk '{print $1}' | awk -F"=" '{print $2}'|head -1)
    if [ "$configFile" != "" ]; then
      value=$(cat $configFile | grep "evictionHard:" -a4 | grep imagefs.available | egrep -o "[0-9]+")
    fi
  fi

  if [ "$value" -gt 5 ]; then
    echo host_kubelet_eviction_config error "evictionHard: imagefs.available is greater than 5%"
    return 0
  fi

  echo host_kubelet_eviction_config ok "-"
}

function check_kubelet_eviction_soft_config() {
  value=$(ps aux | grep /usr/bin/kubelet | egrep -o  "eviction-soft=imagefs.available<([0-9]+)" | awk -F"<" '{print $2}')
  if [ "$value" == "" ]; then
    configFile=$(ps aux | grep "/usr/bin/kubelet" | egrep -o  "\--config=.+" | awk '{print $1}' | awk -F"=" '{print $2}'|head -1)
    if [ "$configFile" != "" ]; then
      value=$(cat $configFile | grep "evictionSoft:" -a4 | grep imagefs.available | egrep -o "[0-9]+")
    fi
  fi

  if [ "$value" -gt 15 ]; then
    echo host_kubelet_eviction_config error "evictionSoft: imagefs.available is greater than 15%"
    return 0
  fi

  echo host_kubelet_evictionSoft_config ok "-"
}


check_cri_status
check_container_number
check_image_number
check_data_dir
check_kubelet_status
check_firewall
check_resolved
check_chronyd
check_cri_notify
check_kubelet_eviction_config
check_kubelet_eviction_soft_config
