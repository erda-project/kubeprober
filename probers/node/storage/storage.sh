#!/bin/bash

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


check_mount_info