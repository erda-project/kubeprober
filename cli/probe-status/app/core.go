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
	"os"
	"path/filepath"
	"strings"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sRestClient client.Client
)

// NewCmdProbeStatusManager creates a *cobra.Command object with default parameters
func NewCmdProbeStatusManager(stopCh <-chan struct{}) *cobra.Command {
	var clusterName string
	var status string
	cmd := &cobra.Command{
		Use:   "probe-status",
		Short: "Launch probe-status",
		Long:  "Launch probe-status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clusterName == "" {
				return GetProbeStatusLocal(status)
			} else {
				return GetProbeStatusSpecifyCluster(clusterName, status)
			}
		},
	}
	cmd.PersistentFlags().StringVarP(&clusterName, "cluster", "c", "", "Print probestatus of specify cluster")
	cmd.PersistentFlags().StringVarP(&status, "status", "s", "", "Print probestatus of specify status [PASS, ERROR, INFO, WARN]")
	return cmd
}

func GetProbeStatusLocal(status string) error {
	var err error
	var probeNames []string
	probeStatus := &kubeproberv1.ProbeStatusList{}
	probes := &kubeproberv1.ProbeList{}
	if err = k8sRestClient.List(context.Background(), probeStatus, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	if err = k8sRestClient.List(context.Background(), probes, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	for _, i := range probes.Items {
		probeNames = append(probeNames, i.Name)
	}
	table := uitable.New()
	table.MaxColWidth = 45
	table.Wrap = true
	table.AddRow("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	//tbl := table.New("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	for _, i := range probeStatus.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				if string(j.Status) == status && status != "" {
					table.AddRow(i.Name, j.Name, j.Status, strings.TrimSpace(j.Message), j.LastRun.Format("2006-01-02 15:04:05"))
				}
				if status == "" {
					table.AddRow(i.Name, j.Name, j.Status, strings.TrimSpace(j.Message), j.LastRun.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}
	fmt.Println(table)
	return nil
}

func GetProbeStatusSpecifyCluster(clusterName string, status string) error {
	var err error
	var c client.Client
	var probeNames []string
	probeStatus := &kubeproberv1.ProbeStatusList{}
	probes := &kubeproberv1.ProbeList{}

	cluster := &kubeproberv1.Cluster{}
	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterName,
	}, cluster); err != nil {
		fmt.Printf("Get cluster info error: %+v\n", err)
		return err
	}

	c, err = GenerateProbeClient(cluster)
	if err != nil {
		return err
	}
	if err = c.List(context.Background(), probeStatus, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	if err = c.List(context.Background(), probes, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	for _, i := range probes.Items {
		probeNames = append(probeNames, i.Name)
	}
	//tbl := table.New("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	table := uitable.New()
	table.MaxColWidth = 45
	table.Wrap = true
	table.AddRow("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	for _, i := range probeStatus.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				if string(j.Status) == status && status != "" {
					table.AddRow(i.Name, j.Name, j.Status, strings.TrimSpace(j.Message), j.LastRun.Format("2006-01-02 15:04:05"))
				}
				if status == "" {
					table.AddRow(i.Name, j.Name, j.Status, strings.TrimSpace(j.Message), j.LastRun.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}
	fmt.Println(table)
	return nil
}
func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		userHomeDir = ""
	}
	kubeConfig := filepath.Join(userHomeDir, ".kube", "config")
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			klog.Errorf("[remote dialer server] get kubernetes client config error: %+v\n", err)
			return
		}
	}

	scheme := runtime.NewScheme()
	kubeproberv1.AddToScheme(scheme)
	k8sRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return
	}
}

func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
