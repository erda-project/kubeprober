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

package main

import (
	"github.com/sirupsen/logrus"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	status "github.com/erda-project/kubeprober/pkg/probe-status"
)

func main() {
	report := []kubeprobev1.ProbeCheckerStatus{
		{
			Name:    "checker1",
			Status:  kubeprobev1.CheckerStatusInfo,
			Message: "checker1 status is info",
		},
		{
			Name:    "checker2",
			Status:  kubeprobev1.CheckerStatusInfo,
			Message: "checker2 status is info",
		},
		{
			Name:    "checker3",
			Status:  kubeprobev1.CheckerStatusError,
			Message: "checker3 is error",
		},
	}
	if err := status.ReportProbeStatus(report); err != nil {
		logrus.Errorf("report probe status failed, err: %v", err)
	}
}
