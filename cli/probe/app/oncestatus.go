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

package app

import (
	"context"
	"fmt"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

var OnceStatusCmd = &cobra.Command{
	Use:   "oncestatus",
	Short: "Print One-time probe status of remote cluster or local cluster",
	Long:  "Print One-probe status of remote cluster or local cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isList {
			return GetHistoryOnceProbeStatus(clusterName)
		} else {
			return GetOnceProbeStatus(clusterName, onceID)
		}
	},
}

func GetHistoryOnceProbeStatus(clusterName string) error {
	var err error

	cluster := &kubeproberv1.Cluster{}
	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterName,
	}, cluster); err != nil {
		fmt.Printf("Get cluster info error: %+v\n", err)
		return err
	}

	//just print once probe status
	table := uitable.New()
	table.MaxColWidth = 45
	table.Wrap = true
	table.AddRow("ID", "PROBES", "CREATETIMN", "FINISHTIME")
	for _, i := range cluster.Status.OnceProbeList {
		table.AddRow(i.ID, i.Probes, i.CreateTime, i.FinishTime)
	}
	fmt.Println(table)
	return nil
}

func GetOnceProbeStatus(clusterName string, id string) error {
	var err error
	var c client.Client
	var onceId string

	cluster := &kubeproberv1.Cluster{}
	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterName,
	}, cluster); err != nil {
		fmt.Printf("Get cluster info error: %+v\n", err)
		return err
	}

	namespace := cluster.Spec.ClusterConfig.ProbeNamespaces
	onceList := cluster.Status.OnceProbeList

	if id == "" {
		onceId = onceList[len(onceList)-1].ID
	} else {
		onceId = id
	}

	if c, err = GenerateProbeClient(cluster); err != nil {
		return err
	}

	if err = PrintOnceProbeStatus(c, namespace, onceId); err != nil {
		return err
	}

	return nil
}
