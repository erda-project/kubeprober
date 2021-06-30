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
	item1 := kubeprobev1.ProbeItemStatus{
		ProbeCheckerStatus: kubeprobev1.ProbeCheckerStatus{
			Name:    "probe-item-test1",
			Status:  kubeprobev1.CheckerStatusError,
			Message: "probe-checker2 error",
			LastRun: &now,
		},
		Checkers: []kubeprobev1.ProbeCheckerStatus{
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
		},
	}

	r := probestatus.ReportProbeStatusSpec{
		ProbeName:       probeName,
		ProbeNamespace:  probeNamespace,
		ProbeItemStatus: item1,
	}

	s := kubeprobev1.ProbeStatus{
		Spec: kubeprobev1.ProbeStatusSpec{
			ProbeCheckerStatus: kubeprobev1.ProbeCheckerStatus{
				Name:   probeName,
				Status: kubeprobev1.CheckerStatusInfo,
			},
			Namespace: probeNamespace,
			Detail:    []kubeprobev1.ProbeItemStatus{},
		},
	}

	_, status := mergeProbeStatus(r, s)
	assert.DeepEqual(t, s, status)
}
