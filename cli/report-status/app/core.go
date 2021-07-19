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
	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cli/report-status/options"
	status "github.com/erda-project/kubeprober/pkg/probe-status"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// NewCmdProbeStatusManager creates a *cobra.Command object with default parameters
func NewCmdReportStatusManager(stopCh <-chan struct{}) *cobra.Command {
	ReportStatusOptions := options.NewReportStatusOptions()
	cmd := &cobra.Command{
		Use:   "report-status",
		Short: "Launch report-status",
		Long:  "Launch report-status",
		Run: func(cmd *cobra.Command, args []string) {
			//cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			//	klog.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			//})
			Run(ReportStatusOptions)
		},
	}
	ReportStatusOptions.AddFlags(cmd.Flags())
	return cmd
}

func Run(opts *options.ReportStatusOptions) {
	now := metav1.Now()
	dnsChecker := kubeprobev1.ProbeCheckerStatus{
		Name:    opts.CheckerName,
		Status:  kubeprobev1.CheckerStatus(opts.Status),
		Message: opts.Message,
		LastRun: &now,
	}
	if err := status.ReportProbeStatus([]kubeprobev1.ProbeCheckerStatus{dnsChecker}); err != nil {
		fmt.Printf("%+v\n", err)
	}
}
