package main

import (
	"github.com/sirupsen/logrus"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	status "github.com/erda-project/kubeprober/pkg/probe-status"
)

func main() {
	report := []kubeprobev1.ProbeCheckerStatus{
		{
			Name:   "checker1",
			Status: kubeprobev1.CheckerStatusInfo,
		},
		{
			Name:   "checker2",
			Status: kubeprobev1.CheckerStatusUNKNOWN,
		},
		{
			Name:   "checker3",
			Status: kubeprobev1.CheckerStatusWARN,
		},
		{
			Name:   "checker4",
			Status: kubeprobev1.CheckerStatusError,
		},
	}
	if err := status.ReportProbeStatus(report); err != nil {
		logrus.Errorf("report probe status failed, err: %v", err)
	}
}
