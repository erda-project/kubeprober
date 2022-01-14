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
	"sort"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DINGDING_ALERT_NAME = "dingding"

type Column struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type TableResponse struct {
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Type    string          `json:"type"`
}

type TimeSerieResponse struct {
	Tatget     string    `json:"target"`
	Datapoints [][]int64 `json:"datapoints"`
}

func GetClusterList(rw http.ResponseWriter, req *http.Request) {
	var err error
	var listRow [][]interface{}
	clusters, err := k8sclient.GetClusters()
	if err != nil {
		errMsg := fmt.Sprintf("[cluster query] failed to get cluster list: %+v\n", err)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, i := range clusters {
		var list []interface{}
		list = append(list, i.Name)
		list = append(list, i.Spec.K8sVersion)
		list = append(list, i.Status.NodeCount)
		list = append(list, i.Spec.ClusterConfig.ProbeNamespaces)
		list = append(list, i.Status.AttachedProbes)
		list = append(list, i.Status.Checkers)
		list = append(list, i.Status.ExtraStatus["diceDomain"])
		list = append(list, i.Status.ExtraStatus["diceVersion"])
		list = append(list, i.Status.ExtraStatus["diceProto"])
		list = append(list, i.Status.ExtraStatus["idEdge"])
		list = append(list, i.Status.ExtraStatus["idFdpCluster"])
		list = append(list, i.Status.ExtraStatus["storageType"])
		list = append(list, i.Status.ExtraStatus["netdataUsed"])
		list = append(list, i.Status.ExtraStatus["k8sVendor"])
		list = append(list, i.Status.ExtraStatus["masterNode"])
		list = append(list, i.Status.ExtraStatus["lbNode"])
		list = append(list, i.Status.ExtraStatus["osImages"])
		list = append(list, i.Status.ExtraStatus["mysqlHost"])
		list = append(list, i.Status.ExtraStatus["nacosAddr"])
		list = append(list, i.Status.ExtraStatus["podNum"])
		list = append(list, i.Status.ExtraStatus["nsNum"])
		list = append(list, i.Status.ExtraStatus["pvcNum"])
		list = append(list, i.Status.ExtraStatus["pvNum"])
		list = append(list, i.Status.ExtraStatus["serviceNum"])
		list = append(list, i.Status.ExtraStatus["ingressNum"])
		list = append(list, i.Status.ExtraStatus["jobNum"])
		list = append(list, i.Status.ExtraStatus["cronjobNum"])
		list = append(list, i.Status.ExtraStatus["deploymentNum"])
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
			{Text: "ERDADOMAIN", Type: "string"},
			{Text: "ERDAVERSION", Type: "string"},
			{Text: "ERDAPROTO", Type: "string"},
			{Text: "ISEDGE", Type: "string"},
			{Text: "ISFDPCLUSTER", Type: "string"},
			{Text: "STORAGETYPE", Type: "string"},
			{Text: "NETDATAUESD", Type: "string"},
			{Text: "K8SVENDER", Type: "string"},
			{Text: "MASTERNODE", Type: "string"},
			{Text: "LBNODE", Type: "string"},
			{Text: "OSIMAGE", Type: "string"},
			{Text: "MYSQLHOST", Type: "string"},
			{Text: "NACOSADDR", Type: "string"},
			{Text: "PODNUM", Type: "string"},
			{Text: "NSNUM", Type: "string"},
			{Text: "PVCNUM", Type: "string"},
			{Text: "PVNUM", Type: "string"},
			{Text: "SERVICENUM", Type: "string"},
			{Text: "INGRESSNUM", Type: "string"},
			{Text: "JOBNUM", Type: "string"},
			{Text: "CRONJOBNUM", Type: "string"},
			{Text: "DEPLOYMENTNUM", Type: "string"},
			{Text: "HEARTBEATTIME", Type: "string"},
		},
		Rows: listRow,
		Type: "table",
	}

	if err := json.NewEncoder(rw).Encode([]TableResponse{resp}); err != nil {
		klog.Errorf("json encode for cluster list error: %+v\n", err)
	}
}

func GetAlertStatistic(rw http.ResponseWriter, req *http.Request) {
	var err error
	var listRow [][]int64
	alert := &kubeproberv1.Alert{}

	if err = k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      DINGDING_ALERT_NAME,
	}, alert); err != nil {
		errMsg := fmt.Sprintf("[alert statistic query] failed to get dingding alert statistic: %+v\n", err)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	l, _ := time.LoadLocation("Asia/Shanghai")
	for k, v := range alert.Status.AlertCount {
		var list []int64

		ts, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("%s 23:59:59", k), l)
		list = append(list, int64(v))
		list = append(list, ts.Unix()*1000)
		listRow = append(listRow, list)
	}
	sort.Slice(listRow, func(i, j int) bool {
		return listRow[i][1] < listRow[j][1]
	})
	resp := TimeSerieResponse{
		Tatget:     "count",
		Datapoints: listRow,
	}

	if err := json.NewEncoder(rw).Encode([]TimeSerieResponse{resp}); err != nil {
		klog.Errorf("json encode for cluster list error: %+v\n", err)
	}
}
