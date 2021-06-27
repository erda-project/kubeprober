package main

import (
	"github.com/sirupsen/logrus"

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
	status "github.com/erda-project/kubeprober/pkg/probe-status"
)

func main() {
	report := []probev1alpha1.ProbeCheckerStatus{
		{
			Name:   "checker1",
			Status: probev1alpha1.CheckerStatusInfo,
		},
		{
			Name:   "checker2",
			Status: probev1alpha1.CheckerStatusUNKNOWN,
		},
		{
			Name:   "checker3",
			Status: probev1alpha1.CheckerStatusWARN,
		},
		{
			Name:   "checker4",
			Status: probev1alpha1.CheckerStatusError,
		},
	}
	if err := status.ReportProbeStatus(report); err != nil {
		logrus.Errorf("report probe status failed, err: %v", err)
	}
}
