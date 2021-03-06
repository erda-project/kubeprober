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

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

const (
	probeNamespace = "default"
	probeName      = "probe-link-test1"
)

func TestUpdateProbeStatus(t *testing.T) {
	now := metav1.Now()
	checker := []kubeproberv1.ProbeCheckerStatus{
		{
			Name:    "probe-checker1",
			Status:  kubeproberv1.CheckerStatusInfo,
			LastRun: &now,
		},
		{
			Name:    "probe-checker2",
			Status:  kubeproberv1.CheckerStatusError,
			Message: "probe-checker2 error",
			LastRun: &now,
		},
	}

	r := kubeproberv1.ReportProbeStatusSpec{
		ProbeName:      probeName,
		ProbeNamespace: probeNamespace,
		Checkers:       checker,
	}

	s := kubeproberv1.ProbeStatus{
		Spec: kubeproberv1.ProbeStatusSpec{
			Checkers: checker,
		},
	}

	_, status := mergeProbeStatus(r, s)
	assert.DeepEqual(t, s, status)
}
