/*
Copyright 2020 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"github.com/erda-project/kubeprobe/cmd/probe-master/options"
	"github.com/erda-project/kubeprobe/pkg/probe-agent/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	// +kubebuilder:scaffold:imports
)

var (
//scheme   = runtime.NewScheme()
//setupLog = ctrl.Log.WithName("setup")
//
//restConfigQPS   = flag.Int("rest-config-qps", 30, "QPS of rest config.")
//restConfigBurst = flag.Int("rest-config-burst", 50, "Burst of rest config.")
)

func init() {
	//_ = clientgoscheme.AddToScheme(scheme)
	//_ = kubeprobev1.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

// NewCmdYurtAppManager creates a *cobra.Command object with default parameters
func NewCmdProbeAgentManager(stopCh <-chan struct{}) *cobra.Command {
	ProbeMasterOptions := options.NewProbeMasterOptions()
	cmd := &cobra.Command{
		Use:   "probe-agent",
		Short: "Launch probe-agent",
		Long:  "Launch probe-agent",
		Run: func(cmd *cobra.Command, args []string) {
			if ProbeMasterOptions.Version {
				//fmt.Printf("%s: %#v\n", "probe-master", projectinfo.Get())
				return
			}

			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			Run(ProbeMasterOptions)
		},
	}

	ProbeMasterOptions.AddFlags(cmd.Flags())
	return cmd
}

func Run(opts *options.ProbeMasterOptions) {

	ctx := context.Background()
	client.Start(ctx, &client.Config{
		Debug:                   false,
		CollectClusterInfo:      true,
		ClusterDialEndpoint:     "ws://127.0.0.1:8088/clusteragent/connect",
		ClusterHeatBeatEndpoint: "http://127.0.0.1:8088/heartbeat",
		ClusterKey:              "moon",
		SecretKey:               "mmon",
		K8SApiServerAddr:        "127.0.0.1:55794",
	})
}
