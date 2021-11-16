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
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sRestClient client.Client
)

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
	clientgoscheme.AddToScheme(scheme)
	k8sRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return
	}
}

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print probe status of remote cluster or local cluster",
	Long:  "Print probe status of remote cluster or local cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return GetProbeStatus(clusterName, status)
	},
}

func GetProbeStatus(clusterName string, status string) error {
	var err error
	var c client.Client
	var probeNames []string

	probeStatusList := &kubeproberv1.ProbeStatusList{}
	probeList := &kubeproberv1.ProbeList{}

	if clusterName == "" {
		c = k8sRestClient
	} else {
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
	}
	if err = c.List(context.Background(), probeStatusList, client.InNamespace("kubeprober")); err != nil {
		return err
	}
	if err = c.List(context.Background(), probeList, client.InNamespace("kubeprober")); err != nil {
		return err
	}
	//just print cron probe status
	for _, i := range probeList.Items {
		if i.Spec.Policy.RunInterval > 0 {
			probeNames = append(probeNames, i.Name)
		}
	}
	table := uitable.New()
	table.MaxColWidth = 45
	table.Wrap = true
	table.AddRow("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	d, _ := time.ParseDuration("-4h")
	oneDayAgo := time.Now().Add(d)
	for _, i := range probeStatusList.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				if j.LastRun == nil {
					j.LastRun = &metav1.Time{Time: time.Now()}
				}
				if j.LastRun.Before(&metav1.Time{Time: oneDayAgo}) {
					continue
				}
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

func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
