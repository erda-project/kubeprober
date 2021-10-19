#!/bin/bash

datadisk=$(kubectl get cm dice-tools-info -o jsonpath={.data.DATA_DISK_MOUNT_POINT})

kubectl probe node /checkers/storage/storage.sh $datadisk