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
	"github.com/spf13/viper"
	"strings"

	"github.com/erda-project/kubeprober/cmd/probe-tunnel/options"
	client "github.com/erda-project/kubeprober/pkg/probe-tunnel/tunnel"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	//+kubebuilder:scaffold:imports
)

// NewCmdProbeTunnelManager creates a *cobra.Command object with default parameters
func NewCmdProbeTunnelManager(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe-agent",
		Short: "Launch probe-agent",
		Long:  "Launch probe-agent",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper.AutomaticEnv()

			// Read from config file
			configFile := options.ProbeTunnelConf.ConfigFile
			if configFile != "" {
				logrus.Infof("read config file: %s", configFile)
				viper.SetConfigFile(configFile)
				if err := viper.ReadInConfig(); err != nil {
					return fmt.Errorf("failed to read config file %s: %w", configFile, err)
				}
				viper.WatchConfig()
			}

			bindFlags(cmd, viper.GetViper())

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				logrus.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			return Run()
		},
	}

	options.ProbeTunnelConf.AddFlags(cmd.Flags())
	return cmd
}

func Run() error {
	opts := options.ProbeTunnelConf
	ctx := context.Background()
	client.Start(ctx, &client.Config{
		Debug:           false,
		ProbeMasterAddr: opts.ProbeMasterAddr,
		ClusterName:     opts.ClusterName,
		SecretKey:       opts.SecretKey,
	})
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
