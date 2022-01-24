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

	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	dialclient "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client"
)

func ClusterConsole(rw http.ResponseWriter, req *http.Request) {
	// TODO make blow correct
	vars := mux.Vars(req)
	clusterName := vars["clusterName"]

	cluster, err := k8sclient.GetCluster(clusterName)
	if err != nil {
		errMsg := fmt.Sprintf("[cluster console] failed to list cluster with name: %s", clusterName)
		logrus.Errorf(errMsg)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		errMsg := fmt.Sprintf("[cluster console] failed to find cluster with name: %s\n", clusterName)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	token := cluster.Spec.ClusterConfig.Token
	if token == "" {
		errMsg := fmt.Sprintf("[cluster console] invalid token for cluster with name: %s\n", clusterName)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	t, err := base64.StdEncoding.DecodeString(cluster.Spec.ClusterConfig.Token)
	if err != nil {
		errMsg := fmt.Sprintf("[cluster console] invalid token for cluster: %s\n", clusterName)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	token = string(t)

	clusterclient, err := dialclient.GenerateProbeClient(cluster)
	if err != nil {
		errMsg := fmt.Sprintf("[cluster console] invalid token for cluster with name: %s\n", clusterName)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	podList := &v1.PodList{}

	err = clusterclient.List(context.Background(), podList,
		client.InNamespace(cluster.Spec.ClusterConfig.ProbeNamespaces),
		client.MatchingLabels{"app": "probe-agent"})
	if err != nil {
		errMsg := fmt.Sprintf("[cluster console] failed to find probe-agent pod for cluster with name: %s\n", clusterName)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		vars := url.Values{}
		vars.Add("container", "probe-agent")
		vars.Add("stdout", "1")
		vars.Add("stdin", "1")
		vars.Add("stderr", "1")
		vars.Add("tty", "1")
		vars.Add("command", "kubectl-shell.sh")
		vars.Add("command", token)

		path := fmt.Sprintf("/api/k8s/clusters/%s/api/v1/namespaces/%s/pods/%s/exec", clusterName, pod.Namespace, pod.Name)

		req.URL.Path = path
		req.URL.RawQuery = vars.Encode()

		a.ServeHTTP(rw, req)
		return
	}

	logrus.Errorf("failed to find a ready probe-agent pod for cluster %s", clusterName)
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Write(apistructs.NewSteveError(apistructs.ServerError,
		fmt.Sprintf("cluster %s does not have a ready probe-agent pod", clusterName)).JSON())
	return
}
