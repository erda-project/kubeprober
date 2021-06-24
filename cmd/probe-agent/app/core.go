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
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/erda-project/kubeprober/cmd/probe-agent/options"
	"github.com/erda-project/kubeprober/cmd/probe-agent/webserver"
	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
	"github.com/erda-project/kubeprober/pkg/probe-agent/controllers"
	client "github.com/erda-project/kubeprober/pkg/probe-agent/tunnel"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(probev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// NewCmdProbeAgentManager creates a *cobra.Command object with default parameters
func NewCmdProbeAgentManager(stopCh <-chan struct{}) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "probe-agent",
		Short: "Launch probe-agent",
		Long:  "Launch probe-agent",
		Run: func(cmd *cobra.Command, args []string) {
			if options.ProbeAgentConf.Version {
				//fmt.Printf("%s: %#v\n", "probe-master", projectinfo.Get())
				return
			}

			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			Run(options.ProbeAgentConf)
		},
	}

	options.ProbeAgentConf.AddFlags(cmd.Flags())
	err := options.ProbeAgentConf.ValidateOptions()
	if err != nil {
		panic(err)
	}
	err = options.ProbeAgentConf.PostConfig()
	if err != nil {
		panic(err)
	}
	return cmd
}

func Run(opts *options.ProbeAgentOptions) {
	zapopt := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapopt)))

	// if debug probe agent, disable tunnel service
	if !opts.ProbeAgentDebug {
		ctx := context.Background()
		go client.Start(ctx, &client.Config{
			Debug:           false,
			ProbeMasterAddr: opts.ProbeMasterAddr,
			ClusterName:     opts.ClusterName,
			SecretKey:       opts.SecretKey,
		})
	}

	// listwatch pod for failed probe pod, listwatch cronjob for reconcile
	// TODO: add list label selector in related controller & merge them here
	newCacheFunc := cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&corev1.Pod{}: {
				Label: labels.SelectorFromSet(labels.Set{probev1alpha1.LabelKeyApp: probev1alpha1.LabelValueApp}),
			},
			&batchv1beta1.CronJob{}: {
				Label: labels.SelectorFromSet(labels.Set{probev1alpha1.LabelKeyApp: probev1alpha1.LabelValueApp}),
			},
		},
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     opts.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: opts.HealthProbeAddr,
		LeaderElection:         opts.EnableLeaderElection,
		LeaderElectionID:       "probe-agent",
		// TODO: use the probe controller running namespace
		// Namespace: "default",
		NewCache: newCacheFunc,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ProbeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Probe")
		os.Exit(1)
	}

	if err = (&controllers.ProbeStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProbeResult")
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

	setupLog.Info("starting probe server")
	s := webserver.NewServer(mgr.GetClient(), opts.ProbeListenAddr)
	s.Start()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
