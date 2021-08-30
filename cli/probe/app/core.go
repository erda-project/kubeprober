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
	"fmt"

	"github.com/spf13/cobra"
)

var (
	clusterName string
	status      string
	probes      string
	onceID      string
	isList      bool
)

func init() {
	OnceCmd.PersistentFlags().StringVarP(&clusterName, "cluster", "c", "", "Name of specify cluster")
	OnceCmd.PersistentFlags().StringVarP(&probes, "probe", "p", "", "Probe name")

	StatusCmd.PersistentFlags().StringVarP(&clusterName, "cluster", "c", "", "Name of specify cluster")
	StatusCmd.PersistentFlags().StringVarP(&status, "status", "s", "", "Status of probe [PASS, ERROR, INFO, WARN]")

	OnceStatusCmd.PersistentFlags().StringVarP(&clusterName, "cluster", "c", "", "Name of specify cluster")
	OnceStatusCmd.PersistentFlags().StringVarP(&onceID, "id", "i", "", "Id of one-time probe, print laste one-time default")
	OnceStatusCmd.PersistentFlags().BoolVarP(&isList, "list", "l", false, "List history once-time probe status")
}

// NewCmdProbeStatusManager creates a *cobra.Command object with default parameters
func NewCmdProbeStatusManager(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use: "kubectl-probe",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Kubeprober CLI Version: v0.0.3 -- HEAD")
		},
	}
	return cmd
}
