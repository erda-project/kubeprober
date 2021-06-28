package controllers

import (
	"testing"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
	"gotest.tools/assert"
)

const (
	probeNamespace = "default"
	probeName      = "probe-link-test1"
)

func TestUpdateProbeStatus(t *testing.T) {
	pis := kubeprobev1.ProbeItemStatus{
		ProbeCheckerStatus: kubeprobev1.ProbeCheckerStatus{
			Name:   "probe-item-test1",
			Status: kubeprobev1.CheckerStatusInfo,
		},
		Checkers: []kubeprobev1.ProbeCheckerStatus{
			{
				Name:   "probe-item-test1",
				Status: kubeprobev1.CheckerStatusInfo,
			},
		},
	}

	r := probestatus.ReportProbeStatusSpec{
		ProbeName:       probeName,
		ProbeNamespace:  probeNamespace,
		ProbeItemStatus: pis,
	}
	s := kubeprobev1.ProbeStatus{
		Spec: kubeprobev1.ProbeStatusSpec{
			ProbeCheckerStatus: kubeprobev1.ProbeCheckerStatus{
				Name:   probeName,
				Status: kubeprobev1.CheckerStatusInfo,
			},
			Namespace: probeNamespace,
			Detail:    []kubeprobev1.ProbeItemStatus{pis},
		},
	}

	_, status := mergeProbeStatus(r, s)
	_, status = mergeProbeStatus(r, s)
	assert.DeepEqual(t, s, status)
}
