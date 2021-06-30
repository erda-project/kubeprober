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
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cmd/probe-agent/options"
	"github.com/erda-project/kubeprober/cmd/probe-agent/webserver"
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
	utilruntime.Must(kubeprobev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// NewCmdProbeAgentManager creates a *cobra.Command object with default parameters
func NewCmdProbeAgentManager(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe-agent",
		Short: "Launch probe-agent",
		Long:  "Launch probe-agent",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			// enable using dashed notation in flags and underscores in env
			viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			viper.AutomaticEnv()

			if err := options.ProbeAgentConf.LoadConfig(); err != nil {
				return err
			}

			err := options.ProbeAgentConf.ValidateOptions()
			if err != nil {
				return fmt.Errorf("validate option failed, error: %v", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				// klog.V(0).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
				logrus.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			return doRun()
		},
	}

	options.ProbeAgentConf.AddFlags(cmd.Flags())
	return cmd
}

func doRun() error {
	ctx := signals.SetupSignalHandler()

	// receive config file update events over a channel
	confUpdateChan := make(chan struct{}, 1)

	viper.OnConfigChange(func(evt fsnotify.Event) {
		if evt.Op&fsnotify.Write == fsnotify.Write || evt.Op&fsnotify.Create == fsnotify.Create {
			confUpdateChan <- struct{}{}
		}
	})

	// start the operator in a goroutine
	errChan := make(chan error, 1)
	ctx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	go func() {
		err := startOperator(ctx)
		if err != nil {
			logrus.Errorf("start operator failed, error: %v", err)
		}
		errChan <- err
	}()

	// watch for events
	for {
		select {
		case err := <-errChan: // operator failed
			logrus.Errorf("shutting down due to error: %v", err)
			return err
		case <-ctx.Done(): // signal received
			logrus.Infof("shutting down due to signal")
			return nil
		case <-confUpdateChan: // config file updated
			logrus.Infof("shutting down to apply updated configuration")
			return nil
		}
	}
}

func startOperator(ctx context.Context) error {
	opts := options.ProbeAgentConf
	zapopt := zap.Options{
		Development: true,
		Level:       zapcore.Level(-opts.DebugLevel),
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapopt)))

	setupLog.V(int(opts.DebugLevel)).Info("", "log level", opts.DebugLevel)
	// setupLog.Info("probe agent", "config", opts)

	// if debug probe agent, disable tunnel service
	if !opts.AgentDebug {
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
				Label: labels.SelectorFromSet(labels.Set{kubeprobev1.LabelKeyApp: kubeprobev1.LabelValueApp}),
			},
			&batchv1beta1.CronJob{}: {
				Label: labels.SelectorFromSet(labels.Set{kubeprobev1.LabelKeyApp: kubeprobev1.LabelValueApp}),
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
		Namespace:              opts.GetNamespace(),
		NewCache:               newCacheFunc,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return fmt.Errorf("start manager failed, error: %v", err)
	}

	if err = (&controllers.ProbeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Probe")
		return fmt.Errorf("create probe controller failed, error: %v", err)
	}

	if err = (&controllers.ProbeStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProbeResult")
		return fmt.Errorf("create probe status controller failed, error: %v", err)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return fmt.Errorf("set up health check failed, error: %v", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return fmt.Errorf("set ready check failed, error: %v", err)
	}

	setupLog.Info("starting probe server")
	s := webserver.NewServer(mgr.GetClient(), opts.ProbeListenAddr)
	s.Start()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return fmt.Errorf("running managet failed, error: %v", err)
	}
	setupLog.Info("start manager successfully")
	return nil
}
