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
	"flag"
	"os"
	"time"

	"github.com/erda-project/kubeprober/pkg/probe-master/controller"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cmd/probe-master/options"
	server "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-server"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	restConfigQPS   = flag.Int("rest-config-qps", 30, "QPS of rest config.")
	restConfigBurst = flag.Int("rest-config-burst", 50, "Burst of rest config.")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kubeproberv1.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

// NewCmdProbeMasterManager creates a *cobra.Command object with default parameters
func NewCmdProbeMasterManager(stopCh <-chan struct{}) *cobra.Command {
	ProbeMasterOptions := options.NewProbeMasterOptions()
	cmd := &cobra.Command{
		Use:   "probe-master",
		Short: "Launch probe-master",
		Long:  "Launch probe-master",
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

	optts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&optts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     opts.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: opts.HealthProbeAddr,
		LeaderElection:         opts.EnableLeaderElection,
		LeaderElectionID:       "probe-master",
		//CertDir:                "config/cert/", //used to develop in local
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	if err = (&controller.ClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cluster")
		os.Exit(1)
	}

	if err = (&controller.ProbeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Probe")
		os.Exit(1)
	}

	if err = (&kubeproberv1.Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Cluster")
		os.Exit(1)
	}

	if err = (&kubeproberv1.Probe{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Probe")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	//start remote cluster dialer
	klog.Infof("starting probe-master remote dialer server on :8088")
	go server.Start(ctx, &server.Config{
		Debug:   false,
		Timeout: 0,
		Listen:  opts.ProbeMasterListenAddr,
	})

	setupLog.Info("starting manager")
	time.Sleep(10 * time.Second)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
