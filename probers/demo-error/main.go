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
			Name:    "checker2",
			Status:  kubeprobev1.CheckerStatusUNKNOWN,
			Message: "UNKNOWN: non info level status message cannot be empty",
		},
		{
			Name:    "checker3",
			Status:  kubeprobev1.CheckerStatusWARN,
			Message: "WARN: non info level status message cannot be empty",
		},
		{
			Name:    "checker4",
			Status:  kubeprobev1.CheckerStatusError,
			Message: "ERROR: non info level status message cannot be empty, message: checker4 run failed because network is timeout",
		},
	}
	if err := status.ReportProbeStatus(report); err != nil {
		logrus.Errorf("report probe status failed, err: %v", err)
	}
}
