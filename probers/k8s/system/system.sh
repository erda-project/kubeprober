#!/bin/bash

function is_deployment_ready() {
    if [[ $# == 1 ]]; then
        namespace=default
    else
        namespace=$2
    fi
    replicas=$(kubectl get deploy $1 -n $namespace -o jsonpath={.spec.replicas} 2> /dev/null)
    if [[ $? -ne 0 ]]; then        # maybe some components not exists anymore or renamed in some versions
        return
    fi
    if [[ $replicas == 0 ]]; then
        echo $1 info "spec.replicas is $replicas"
    fi
    ready_replicas=$(kubectl get deploy $1 -n $namespace -o jsonpath={.status.readyReplicas})
    if [[ "$replicas" != "$ready_replicas" ]]; then
        echo $1 error "ready_replicas != spec.replicas"
    else
        echo $1 ok
    fi
}

function is_daemonset_ready() {
    if [[ $# == 1 ]]; then
        namespace=default
    else
        namespace=$2
    fi
    desiredNumberScheduled=$(kubectl get ds $1 -n $namespace -o jsonpath={.status.desiredNumberScheduled} 2> /dev/null)
    if [ $? -ne 0 ]; then        # maybe some components not exists anymore or renamed in some versions
        return
    fi
    numberReady=$(kubectl get ds $1 -n $namespace -o jsonpath={.status.numberReady})
    observedGeneration=$(kubectl get ds $1 -n $namespace -o jsonpath={.status.observedGeneration})
    if [[ -z $observedGeneration ]]; then
        echo $1 error "empty observedGeneration"
        return
    fi
    if [ $desiredNumberScheduled != $numberReady ]; then
        echo $1 error "desiredNumberScheduled != numberReady"
    else
        echo $1 ok
    fi
}

# check whether k8s control plane components healthy
function is_components_healthy () {
    if kubectl get componentstatuses -o jsonpath="{range .items[*]}{range .conditions[*]}{.status}{'\n'}{end}{end}" | grep False > /dev/null 2>&1 ; then
        echo k8s-componentsstatus error "k8s components not healthy"
    else
        echo k8s-componentsstatus ok
    fi
}

# check whether k8s core components status
function check_k8s_status() {
    is_components_healthy

    is_daemonset_ready calico-node kube-system
    is_daemonset_ready coredns kube-system
    is_daemonset_ready kube-proxy kube-system
    is_daemonset_ready nginx-ingress-controller kube-system

    is_deployment_ready calico-kube-controllers kube-system
}

# check k8s node ready status
function check_node_ready() {
    if kubectl get node -o jsonpath='{range .items[*].status}{range .conditions[?(@.type=="Ready")]}{.status}{"\n"}{end}{end}' | grep False > /dev/null 2>&1; then
        echo k8s-nodeready error "NotReady node found"
    else
        echo k8s-nodeready ok
    fi
}

# now only check whether cpu/mem resource & limit set
function check_resource() {
    if [[ $# != 5 ]]; then
        echo $2_$3 error "not enough params"
        return 1
    fi

    res=$(kubectl get $1 -n kube-system $2)
    if [[ "$res" == "" ]]; then
      #不存在的资源调过检查
      return 0
    fi
    res=$(kubectl get $1 -n kube-system $2 -o jsonpath="{.spec.template.spec.containers[?(@.name=='$2')].resources.$3.$4}")
    if [[ "$res" == "" ]]; then
        echo k8s-$2_$3_$4 error "$3_$4 not set"
    else
        echo k8s-$2_$3_$4 ok
    fi
}

# check whether k8s core components' resource set
function check_k8s_components_resources() {
    if [[ "$cluster_name" == terminus-captain || "$cluster_name" == baosheng-edge || "$is_cs" == true ]]; then
        return 0
    fi

    check_resource deploy calico-kube-controllers requests cpu "10m"
    check_resource deploy calico-kube-controllers requests memory "100Mi"
    check_resource deploy calico-kube-controllers limits cpu "1"
    check_resource deploy calico-kube-controllers limits memory "1000Mi"

    check_resource ds calico-node requests cpu "10m"
    check_resource ds calico-node requests memory "100Mi"
    check_resource ds calico-node limits cpu "1"
    check_resource ds calico-node limits memory "1000Mi"

    check_resource ds kube-proxy requests cpu "10m"
    check_resource ds kube-proxy requests memory "64Mi"
    check_resource ds kube-proxy limits cpu "1"
    check_resource ds kube-proxy limits memory "256Mi"

    check_resource ds coredns requests cpu "100m"
    check_resource ds coredns requests memory "70Mi"
    check_resource ds coredns limits cpu "1"
    check_resource ds coredns limits memory "170Mi"
}

# check where dice volume's path is /data
function check_dicevolume_path() {
    if [[ "$is_cs" == true ]]; then
        return 0
    fi

    hostpath=$(kubectl get sc dice-local-volume -o jsonpath='{.parameters.hostpath}')
    if [[ $hostpath == "/data" ]]; then
        echo dice-volume ok
    else
        echo dice-volume error "dice volume hostpath should be /data"
    fi
}

# check erda node lables
function check_node_label() {
    any_lable_n=$(kubectl get node --show-labels | grep  "any=")
    # cannot catain label 'any'
    if [[ $any_lable_n != "" ]]
    then
        echo k8s_node_label error "there are some node with any label"
        return
    fi

    # must contain 'org-' label
    total_count=$(kubectl get node | grep -v NAME | wc -l)
    org_count=$(kubectl get node --show-labels | grep org- | wc -l)
    if [[ $total_count != $org_count ]]
    then
        echo k8s_node_label error "some node do not have org lable"
        return
    fi
    # at least three nodes contain labels 'location-cluster-service' & 'stateful-service'
    location_label_count=$(kubectl get node --show-labels | grep  "location-cluster-service" | wc -l)
    location_stateful_label_count=$(kubectl get node --show-labels | grep  "location-cluster-service" | grep "stateful-service" | wc -l)
    if [[ $location_label_count -gt 0  ]] && [[ $location_stateful_label_count -lt 3  ]]
    then
        echo k8s_node_label error "location-cluster-service / stateful-service tag less than 3"
    fi
    echo k8s_node_label ok
}

# check whether node is cordon
function check_node_cordon() {
    cordon_node=$(kubectl get nodes -o wide | grep -v NAME  | grep SchedulingDisabled | awk '{print $1}' | tr "\n" " ")
    if [ "$cordon_node" != "" ] >/dev/null 2>&1; then
        echo check_node_cordon error "there exists node: $cordon_node SchedulingDisabled"
    else
        echo check_node_cordon ok
    fi
}

# check whether pipeline namespaces leak
function pipeline_namespace_leak() {
    count=`kubectl get ns|grep -i pipeline|wc -l`
    if [[ $count -gt 5000 ]]; then
        echo "pipeline_namespace_leak" error "too many pipeline namespaces:" $count
    else
        echo "pipeline_namespace_leak" ok ""
    fi
}

# check k8s component version consistency
function check_k8s_version() {
    # check kubelet version on all node
    if [[ `kubectl get node|awk '{print $NF}'|grep -v VER|uniq|wc -l` == 1 ]] ; then
        echo "kubelet_version" ok ""
    else
        echo "kubelet_version" error "kubelet version is not same"
    fi

    # check kubelet & server (apiserver, controller, scheduler) version
    kubelet_v=$(kubectl get node --no-headers|head -n 1|awk '{print $NF}')
    server_v=$(kubectl version|grep "Server Version"|awk '{print $5}'|awk -F"\"" '{print $2}')
    kubelet_minor=$(echo "$kubelet_v"|awk -F"." '{print $2}')
    server_minor=$(echo "$server_v"|awk -F"." '{print $2}')
    if [[ "$kubelet_minor" != "$server_minor" ]]; then
      echo "kubelet_server_version" error "kubelet and server(apiserver, controller, scheduler) minor version is different"
    elif [[ "$kubelet_v" != "$server_v" ]]; then
      echo "kubelet_server_version" info "kubelet and server(apiserver, controller, scheduler) version is different"
    fi
}

# check k8s core component status (calico-node, calico, kube-proxy, nginx-ingress-controller)
check_k8s_status
# check whether k8s core components' resource set
check_k8s_components_resources
# check k8s component version consistency
check_k8s_version
# check k8s node ready status
check_node_ready
# check erda node lables
check_node_label
# check whether node is cordon
check_node_cordon
# check whether pipeline namespaces leak
pipeline_namespace_leak
# check where dice volume's path is correct
check_dicevolume_path









