// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package heartbeat

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	addonInfoCm = "dice-addons-info"
	toolsInfoCm = "dice-tools-info"
	getNetdataInfoCmd = "df -h \"/netdata\" | awk -v mp=\"/netdata\" '{if($NF==mp)print $3}'"
)

func getExtraStatus(k8sRestClient client.Client, config *rest.Config) map[string]string {
	s := make(map[string]string)
	cm := &v1.ConfigMap{}
	masterHost := &v1.NodeList{}
	lbHost := &v1.NodeList{}

	var err error
	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterInfoCm,
	}, cm); err != nil {
		klog.Errorf("fail to get dice-cluster-info cm: %+v\n", err)
	}

	s["diceVersion"] = cm.Data["DICE_VERSION"]
	s["k8sVendor"] = cm.Data["KUBERNETES_VENDOR"]
	s["diceProto"] = cm.Data["DICE_PROTOCOL"]
	s["diceDomain"] = cm.Data["DICE_ROOT_DOMAIN"]
	s["idEdge"] = cm.Data["DICE_IS_EDGE"]
	s["idFdpCluster"] = cm.Data["IS_FDP_CLUSTER"]

	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      toolsInfoCm,
	}, cm); err != nil {
		klog.Errorf("fail to get dice-tools-info cm: %+v\n", err)
	}

	s["storageType"] = cm.Data["STORAGE_TYPE"]
	s["storageServers"] = cm.Data["STORAGE_SERVERS"]

	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      addonInfoCm,
	}, cm); err != nil {
		klog.Errorf("fail to get dice-addon-info cm: %+v\n", err)
	}
	s["mysqlHost"] = cm.Data["MYSQL_HOST"]
	s["nacosAddr"] = fmt.Sprintf("%s:%s", cm.Data["MS_NACOS_HOST"], cm.Data["MS_NACOS_PORT"])

	if err = k8sRestClient.List(context.Background(), masterHost, client.MatchingLabels{"node-role.kubernetes.io/master": ""}); err != nil {
		klog.Errorf("fail to get master node: %+v\n", err)
	}

	if err = k8sRestClient.List(context.Background(), lbHost, client.MatchingLabels{"node-role.kubernetes.io/lb": ""}); err != nil {
		klog.Errorf("fail to get lb node: %+v\n", err)
	}
	s["masterNode"] = fmt.Sprintf("%d", len(masterHost.Items))
	s["lbNode"] = fmt.Sprintf("%d", len(lbHost.Items))

	pods := &v1.PodList{}
	var nsenterPodName string
	var stdo string
	var stde string
	if err = k8sRestClient.List(context.Background(), pods, client.InNamespace("kubeprober")); err != nil {
		klog.Errorf("fail to get pod list in kubeproebr for nsenter: %+v\n", err)
	}

	for _, v := range pods.Items {
		if v.Status.Phase == v1.PodRunning && strings.Contains(v.Name, "nsenter") {
			nsenterPodName = v.Name
			break
		}
	}
	if stdo, stde, err = ExecInPod(config, "kubeprober", nsenterPodName, getNetdataInfoCmd, "nsenter"); err != nil {
		klog.Errorf("fail to get netdata info by nsenter: %+v, %+v\n", stde, err)
	}
	s["netdataUsed"] = stdo

	return s
}

func ExecInPod(config *rest.Config, namespace, podName, command, containerName string) (string, string, error) {
	k8sCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", err
	}
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	const tty = false
	req := k8sCli.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).SubResource("exec").Param("container", containerName)
	req.VersionedParams(
		&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     tty,
		},
		scheme.ParameterCodec,
	)

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}
