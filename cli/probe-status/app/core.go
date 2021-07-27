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
	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ProbeRestClient client.Client
)

// NewCmdProbeStatusManager creates a *cobra.Command object with default parameters
func NewCmdProbeStatusManager(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe-status",
		Short: "Launch probe-status",
		Long:  "Launch probe-status",
		RunE: func(cmd *cobra.Command, args []string) error {
			//cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			//	klog.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			//})

			return Run()
		},
	}

	return cmd
}

func Run() error {
	var err error
	var probeNames []string
	probeStatus := &kubeprobev1.ProbeStatusList{}
	probes := &kubeprobev1.ProbeList{}
	if err = ProbeRestClient.List(context.Background(), probeStatus, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	if err = ProbeRestClient.List(context.Background(), probes, client.InNamespace("kubeprober")); err != nil {
		return err
	}

	for _, i := range probes.Items {
		probeNames = append(probeNames, i.Name)
	}
	tbl := table.New("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	for _, i := range probeStatus.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				tbl.AddRow(i.Name, j.Name, j.Status, j.Message, j.LastRun)
			}
		}
	}
	tbl.Print()
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
	kubeprobev1.AddToScheme(scheme)
	ProbeRestClient, err = client.New(config, client.Options{Scheme: scheme})
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
