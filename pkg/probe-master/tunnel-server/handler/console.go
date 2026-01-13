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

package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	dialclient "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client"
)

const (
	probeAgentContainer = "probe-agent"
	execCommandScript   = "kubectl-shell.sh"
)

func ClusterConsole(rw http.ResponseWriter, req *http.Request) {
	clusterName := mux.Vars(req)["clusterName"]
	if clusterName == "" {
		http.Error(rw, "cluster name is required", http.StatusNotFound)
		return
	}

	cluster, err := k8sclient.GetCluster(clusterName)
	if err != nil {
		logrus.Errorf("[cluster console] failed to get cluster %s: %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(rw, fmt.Sprintf("cluster %s not found", clusterName), http.StatusBadRequest)
		return
	}

	token, err := decodeClusterToken(clusterName, cluster.Spec.ClusterConfig.Token)
	if err != nil {
		logrus.Errorf("[cluster console] invalid token for cluster %s: %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	clusterclient, err := dialclient.GenerateProbeClient(cluster)
	if err != nil {
		logrus.Errorf("[cluster console] failed to build k8s client for cluster %s: %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	pod, err := findRunningProbeAgent(req.Context(), clusterclient, cluster.Spec.ClusterConfig.ProbeNamespaces)
	if err != nil {
		logrus.Errorf("[cluster console] failed to list probe-agent pods for cluster %s: %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	if pod == nil {
		logrus.Errorf("failed to find a ready probe-agent pod for cluster %s", clusterName)
		http.Error(rw, fmt.Sprintf("cluster %s does not have a ready probe-agent pod", clusterName), http.StatusInternalServerError)
		return
	}

	req.URL.Path = execURLPath(clusterName, pod.Namespace, pod.Name)
	req.URL.RawQuery = execQuery(token).Encode()

	if proxyManager == nil {
		logrus.Errorf("proxy manager not initialized for cluster %s", clusterName)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	proxyManager.ProxyRequest(rw, req, clusterName)
}

func decodeClusterToken(clusterName, encoded string) (string, error) {
	if encoded == "" {
		return "", fmt.Errorf("empty token for cluster %s", clusterName)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func findRunningProbeAgent(ctx context.Context, c client.Client, namespace string) (*v1.Pod, error) {
	podList := &v1.PodList{}
	err := c.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabels{kubeproberv1.LabelKeyApp: kubeproberv1.LabelValueProbeAgent})
	if err != nil {
		return nil, err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == v1.PodRunning {
			return pod, nil
		}
	}

	return nil, nil
}

func execQuery(token string) url.Values {
	query := url.Values{}
	query.Add("container", probeAgentContainer)
	query.Add("stdout", "1")
	query.Add("stdin", "1")
	query.Add("stderr", "1")
	query.Add("tty", "1")
	query.Add("command", execCommandScript)
	query.Add("command", token)
	return query
}

func execURLPath(clusterName, namespace, podName string) string {
	return fmt.Sprintf("/api/k8s/clusters/%s/api/v1/namespaces/%s/pods/%s/exec", clusterName, namespace, podName)
}
