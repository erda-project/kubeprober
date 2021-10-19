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

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	appv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var OpsCmd = &cobra.Command{
	Use:   "ops",
	Short: "ops tool cli of kubeprober",
	Long:  "ops tool cli of kubeprober",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listAgent {
			return ListAgent()
		}

		if agentImage != "" || agentCpuLimit != "" || agentMemoryLimit != "" {
			return UpdateAgentSetting(agentImage, agentCpuLimit, agentMemoryLimit)
		}
		return func() error {
			fmt.Printf("I am ops tool cli of kubeprober!\n")
			return nil
		}()
	},
}

func ListAgent() error {
	var err error
	clusterList := &kubeproberv1.ClusterList{}
	if err = k8sRestClient.List(context.Background(), clusterList); err != nil {
		fmt.Printf("Get cluster list error: %+v\n", err)
		return err
	}

	table := uitable.New()
	table.MaxColWidth = 70
	table.Wrap = true
	table.AddRow("CLUSTER", "IMAGE", "CPUSET", "MEMORYSET")
	for _, v := range clusterList.Items {
		GetAgentInfo(&v, table)
	}
	fmt.Println(table)
	return nil
}

func GetAgentInfo(cluster *kubeproberv1.Cluster, table *uitable.Table) {
	var err error
	var c client.Client
	var result string

	agentDeploy := &appv1.Deployment{}
	if c, err = GenerateProbeClient(cluster); err != nil {
		result = fmt.Sprintf("%+s\n", err)
		table.AddRow(cluster.Name, result)
		return
	}
	if err = c.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Spec.ClusterConfig.ProbeNamespaces,
		Name:      "probe-agent",
	}, agentDeploy); err != nil {
		result = fmt.Sprintf("%+s\n", err)
		table.AddRow(cluster.Name, result)
		return
	}
	table.AddRow(cluster.Name, agentDeploy.Spec.Template.Spec.Containers[0].Image, agentDeploy.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu(), agentDeploy.Spec.Template.Spec.Containers[0].Resources.Limits.Memory())
}

func UpdateAgentSetting(imageName string, agentCpuLimit string, agentMemoryLimit string) error {
	var err error
	clusterList := &kubeproberv1.ClusterList{}
	if err = k8sRestClient.List(context.Background(), clusterList); err != nil {
		fmt.Printf("Get cluster list error: %+v\n", err)
		return err
	}

	table := uitable.New()
	table.MaxColWidth = 70
	table.Wrap = true
	table.AddRow("CLUSTER", "RESULT")
	for _, v := range clusterList.Items {
		SetAgentSeting(&v, imageName, agentCpuLimit, agentMemoryLimit, table)
	}
	fmt.Println(table)
	return nil
}

func SetAgentSeting(cluster *kubeproberv1.Cluster, imageName string, agentCpuLimit string, agentMemoryLimit string, table *uitable.Table) {
	var err error
	var c client.Client
	var result string

	agentDeploy := &appv1.Deployment{}
	if c, err = GenerateProbeClient(cluster); err != nil {
		result = fmt.Sprintf("%+s\n", err)
		table.AddRow(cluster.Name, result)
		return
	}

	if err = c.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Spec.ClusterConfig.ProbeNamespaces,
		Name:      "probe-agent",
	}, agentDeploy); err != nil {
		result = fmt.Sprintf("%+s\n", err)
		table.AddRow(cluster.Name, result)
		return
	}
	if imageName != "" {
		agentDeploy.Spec.Template.Spec.Containers[0].Image = imageName
	}
	if agentMemoryLimit != "" {
		agentDeploy.Spec.Template.Spec.Containers[0].Resources.Limits[apiv1.ResourceMemory] = resource.MustParse(fmt.Sprintf("%v", agentMemoryLimit))
	}
	if agentCpuLimit != "" {
		agentDeploy.Spec.Template.Spec.Containers[0].Resources.Limits[apiv1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%v", agentCpuLimit))
	}
	if c.Update(context.Background(), agentDeploy); err != nil {
		result = fmt.Sprintf("%+s\n", err)
		table.AddRow(cluster.Name, result)
		return
	}
	table.AddRow(cluster.Name, "SUCCESS")
}
