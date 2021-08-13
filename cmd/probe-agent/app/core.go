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

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cmd/probe-agent/options"
	"github.com/erda-project/kubeprober/cmd/probe-agent/webserver"
	"github.com/erda-project/kubeprober/pkg/probe-agent/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubeproberv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// NewCmdProbeAgentManager creates a *cobra.Command object with default parameters
func NewCmdProbeAgentManager(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe-agent",
		Short: "Launch probe-agent",
		Long:  "Launch probe-agent",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper.AutomaticEnv()

			// Read from config file
			configFile := options.ProbeAgentConf.ConfigFile
			if configFile != "" {
				logrus.Infof("read config file: %s", configFile)
				viper.SetConfigFile(configFile)
				if err := viper.ReadInConfig(); err != nil {
					return fmt.Errorf("failed to read config file %s: %w", configFile, err)
				}
				viper.WatchConfig()
			}

			bindFlags(cmd, viper.GetViper())

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

	// listwatch pod for failed probe pod, listwatch cronjob for reconcile
	// TODO: add list label selector in related controller & merge them here
	newCacheFunc := cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&corev1.Pod{}: {
				Label: labels.SelectorFromSet(labels.Set{kubeproberv1.LabelKeyApp: kubeproberv1.LabelValueApp}),
			},
			&batchv1beta1.CronJob{}: {
				Label: labels.SelectorFromSet(labels.Set{kubeproberv1.LabelKeyApp: kubeproberv1.LabelValueApp}),
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
	s.Start(opts.ProbeMasterAddr, opts.ClusterName)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return fmt.Errorf("running managet failed, error: %v", err)
	}
	setupLog.Info("start manager successfully")
	return nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, "-") {
			// Environment variables can't have dashes in them, so bind them to their equivalent
			// keys with underscores, e.g. --cluster-name to CLUSTER_NAME
			envVar := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			_ = v.BindEnv(f.Name, envVar)
			// Config file keys must be same as flag keys by default, but usually keys with underscores is more widely used.
			// So alias can be bind to the flag, e.g. --cluster-name to cluster_name
			v.RegisterAlias(strings.ReplaceAll(f.Name, "-", "_"), f.Name)
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
