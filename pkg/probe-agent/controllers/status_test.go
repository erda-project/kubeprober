package controllers

import (
	"testing"

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
	"gotest.tools/assert"
)

const (
	probeNamespace = "default"
	probeName      = "probe-link-test1"
)

func TestUpdateProbeStatus(t *testing.T) {
	pis := probev1alpha1.ProbeItemStatus{
		ProbeCheckerStatus: probev1alpha1.ProbeCheckerStatus{
			Name:   "probe-item-test1",
			Status: probev1alpha1.CheckerStatusInfo,
		},
		Checkers: []probev1alpha1.ProbeCheckerStatus{
			{
				Name:   "probe-item-test1",
				Status: probev1alpha1.CheckerStatusInfo,
			},
		},
	}

	r := probestatus.ReportProbeStatusSpec{
		ProbeName:       probeName,
		ProbeNamespace:  probeNamespace,
		ProbeItemStatus: pis,
	}
	s := probev1alpha1.ProbeStatus{
		Spec: probev1alpha1.ProbeStatusSpec{
			ProbeCheckerStatus: probev1alpha1.ProbeCheckerStatus{
				Name:   probeName,
				Status: probev1alpha1.CheckerStatusInfo,
			},
			Namespace: probeNamespace,
			Detail:    []probev1alpha1.ProbeItemStatus{pis},
		},
	}

	_, status := mergeProbeStatus(r, s)
	assert.DeepEqual(t, s, status)
}
