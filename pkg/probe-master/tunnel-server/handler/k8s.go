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
	"encoding/json"
	"fmt"
	"net/http"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Column struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type TableResponse struct {
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Type    string          `json:"type"`
}

func GetClusterList(rw http.ResponseWriter, req *http.Request) {
	var err error
	var listRow [][]interface{}
	clusters := &kubeproberv1.ClusterList{}

	if err = k8sclient.RestClient.List(context.Background(), clusters, client.InNamespace(metav1.NamespaceDefault)); err != nil {
		errMsg := fmt.Sprintf("[cluster query] failed to get cluster list: %+v\n", err)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, i := range clusters.Items {
		var list []interface{}
		list = append(list, i.Name)
		list = append(list, i.Spec.K8sVersion)
		list = append(list, i.Status.NodeCount)
		list = append(list, i.Spec.ClusterConfig.ProbeNamespaces)
		list = append(list, i.Status.AttachedProbes)
		list = append(list, i.Status.Checkers)
		list = append(list, i.Status.HeartBeatTimeStamp)
		listRow = append(listRow, list)
	}
	resp := TableResponse{
		Columns: []Column{
			{Text: "NAME", Type: "string"},
			{Text: "VERSION", Type: "string"},
			{Text: "NODECOUNT", Type: "string"},
			{Text: "PROBENAMESPACE", Type: "string"},
			{Text: "PROBE", Type: "string"},
			{Text: "TOTAL/ERROR", Type: "string"},
			{Text: "HEARTBEATTIME", Type: "string"},
		},
		Rows: listRow,
		Type: "table",
	}

	if err := json.NewEncoder(rw).Encode([]TableResponse{resp}); err != nil {
		klog.Errorf("json encode for cluster list error: %+v\n", err)
	}
}
