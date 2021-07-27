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

package controllers

import (
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

const (
	probeNamespace = "default"
	probeName      = "probe-link-test1"
)

func TestUpdateProbeStatus(t *testing.T) {
	now := metav1.Now()
	checker := []kubeprobev1.ProbeCheckerStatus{
		{
			Name:    "probe-checker1",
			Status:  kubeprobev1.CheckerStatusInfo,
			LastRun: &now,
		},
		{
			Name:    "probe-checker2",
			Status:  kubeprobev1.CheckerStatusError,
			Message: "probe-checker2 error",
			LastRun: &now,
		},
	}

	r := probestatus.ReportProbeStatusSpec{
		ProbeName:      probeName,
		ProbeNamespace: probeNamespace,
		Checkers:       checker,
	}

	s := kubeprobev1.ProbeStatus{
		Spec: kubeprobev1.ProbeStatusSpec{
			Checkers: checker,
		},
	}

	_, status := mergeProbeStatus(r, s)
	assert.DeepEqual(t, s, status)
}
