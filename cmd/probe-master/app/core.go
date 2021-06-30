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
	"flag"
	"os"
	"time"

	"github.com/erda-project/kubeprober/pkg/probe-master/controller"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
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
	_ = kubeprobev1.AddToScheme(scheme)

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

	if err = (&kubeprobev1.Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Cluster")
		os.Exit(1)
	}

	if err = (&kubeprobev1.Probe{}).SetupWebhookWithManager(mgr); err != nil {
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
