// Copyright (c) 2021 Terminus, Inc.
//
// This program is free software: you can use, redistribute, and/or modify
// it under the terms of the GNU Affero General Public License, version 3
// or later ("AGPL"), as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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
