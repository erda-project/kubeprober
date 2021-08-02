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

package probe_status

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

func initEvn() {
	_ = os.Setenv(kubeproberv1.ProbeNamespace, "kubeprober")
	_ = os.Setenv(kubeproberv1.ProbeName, "probe-link-test")
	// _ = os.Setenv(kubeproberv1.ProbeStatusReportUrl, "http://probeagent.kubeprober.svc.cluster.local:8082/probe-status")
	_ = os.Setenv(kubeproberv1.ProbeStatusReportUrl, "http://localhost:8082/probe-status")
}

func TestReportStatus(t *testing.T) {
	initEvn()
	now := metav1.Now()
	checker1 := kubeproberv1.ProbeCheckerStatus{
		Name:    "probe-checker-test1",
		Status:  kubeproberv1.CheckerStatusError,
		Message: "probe-checker-test1 error",
		LastRun: nil,
	}
	checker2 := kubeproberv1.ProbeCheckerStatus{
		Name:    "probe-checker-test2",
		Status:  kubeproberv1.CheckerStatusInfo,
		Message: "probe-checker-test2 info",
		LastRun: &now,
	}
	checkers := []kubeproberv1.ProbeCheckerStatus{checker1, checker2}
	err := ReportProbeStatus(checkers)
	assert.NoError(t, err)
}
